"""Lifecycle tests use real files/processes and substitute Cargo/pnpm/Vite only.

These tests do not compile Rust, run Docker or claim browser HMR coverage.
"""

from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import unittest


RUNNER_PATH = Path(__file__).resolve().parents[1] / "runner.py"
spec = importlib.util.spec_from_file_location("development_runner", RUNNER_PATH)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


SERVICE = r'''
import json, os, signal, socket, subprocess, sys, time
from pathlib import Path
data = Path(os.environ['TEST_DATA'])
role = os.environ['TEST_ROLE']
port = int(os.environ['ET_API_SERVER_PORT' if role == 'backend' else 'WEB_PORT'])
if SERVICE_VERSION == 'start-fail':
    sys.exit(11)
if os.environ.get('TEST_GRANDCHILD') == '1':
    child = subprocess.Popen([sys.executable, '-c',
        'import signal,time;signal.signal(signal.SIGTERM,signal.SIG_IGN);time.sleep(60)'])
    (data / 'grandchild.pid').write_text(str(child.pid))
sock = socket.socket()
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.bind(('127.0.0.1', port))
sock.listen()
with (data / 'starts.jsonl').open('a') as output:
    output.write(json.dumps({'version': SERVICE_VERSION, 'pid': os.getpid()}) + '\n')
while True:
    connection, _ = sock.accept()
    try:
        connection.sendall(b'HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK')
    except OSError:
        pass
    connection.close()
'''

CARGO = r'''
import json, os, sys, time
from pathlib import Path
data = Path(os.environ['TEST_DATA'])
version = Path('easytier-web/src/main.rs').read_text()
with (data / 'builds.jsonl').open('a') as output:
    output.write(json.dumps({'version': version, 'args': sys.argv[1:]}) + '\n')
if version.startswith('slow'):
    (data / 'building-slow').write_text(version)
    (data / 'builder.pid').write_text(str(os.getpid()))
    time.sleep(4)
if version == 'bad':
    sys.exit(7)
profile = 'release' if '--release' in sys.argv else 'debug'
executable = Path(os.environ['CARGO_TARGET_DIR']) / profile / 'easytier-web'
executable.parent.mkdir(parents=True, exist_ok=True)
executable.write_text('#!' + sys.executable + '\nSERVICE_VERSION = ' + repr(version) + '\n' + SERVICE)
executable.chmod(0o755)
'''

PNPM = r'''
import json, os, sys
from pathlib import Path
data = Path(os.environ['TEST_DATA'])
step = 'install' if 'install' in sys.argv else 'codegen'
with (data / 'pnpm.jsonl').open('a') as output:
    output.write(json.dumps({'step': step, 'args': sys.argv[1:]}) + '\n')
if step == 'install':
    if 'baddep' in Path('easytier-web/frontend/package.json').read_text():
        sys.exit(8)
    Path('node_modules').mkdir(exist_ok=True)
else:
    protocol = Path('easytier-proto/proto/common.proto').read_text()
    if protocol == 'badproto':
        sys.exit(9)
    generated = Path('easytier-web/frontend-lib/src/generated/proto')
    generated.mkdir(parents=True, exist_ok=True)
    (generated / 'common.ts').write_text(protocol)
'''


@unittest.skipUnless(os.name == "posix" and Path("/proc").exists() and shutil.which("rsync"),
                     "requires Linux process groups and rsync")
class RunnerLifecycleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="easytier-runner-")
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.data = self.root / "data"
        self.tools = self.root / "tools"
        for directory in (self.source, self.data, self.tools):
            directory.mkdir()
        self.write("Cargo.toml", "[workspace]\nmembers = []\n")
        self.write("Cargo.lock", "# test fixture\n")
        self.write("easytier-web/src/main.rs", "v1")
        self.write("easytier-web/frontend/src/App.vue", "<template>first</template>")
        self.write("easytier-web/frontend/package.json", '{"name":"easytier-frontend"}')
        self.write("easytier-web/frontend-lib/package.json", '{"name":"easytier-frontend-lib"}')
        self.write("pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
        self.write("pnpm-workspace.yaml", "packages: []\n")
        self.write("easytier-proto/proto/common.proto", "proto-v1")
        self.write("easytier-web/frontend-lib/scripts/codegen-proto.mjs", "// fixture\n")
        self.write("deploy/frontend.mjs", "// fixture\n")
        self.executable("cargo", "SERVICE = " + repr(SERVICE) + "\n" + CARGO)
        self.executable("pnpm", PNPM)
        self.executable("node", "SERVICE_VERSION = 'frontend'\n" + SERVICE)
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            self.port = probe.getsockname()[1]
        self.processes = []
        self.output_files = []

    def tearDown(self):
        for process in self.processes:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=6)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
        for output in self.output_files:
            output.close()
        self.temporary.cleanup()

    def write(self, name, text):
        path = self.source / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def executable(self, name, script):
        path = self.tools / name
        path.write_text("#!" + sys.executable + "\n" + script)
        path.chmod(0o755)

    def start(self, role="backend", **extra):
        environment = dict(os.environ)
        environment.update({
            "PATH": str(self.tools) + os.pathsep + environment["PATH"],
            "TEST_DATA": str(self.data), "TEST_ROLE": role,
            "EASYTIER_SOURCE_DIR": str(self.source), "EASYTIER_DATA_DIR": str(self.data),
            "EASYTIER_WATCH": "true", "EASYTIER_POLL_INTERVAL": "0.05",
            "EASYTIER_DEBOUNCE": "0.08", "PYTHONDONTWRITEBYTECODE": "1",
            "ET_API_SERVER_PORT": str(self.port), "ET_API_SERVER_ADDR": "127.0.0.1",
            "WEB_PORT": str(self.port),
        })
        environment.update(extra)
        output = (self.root / f"runner-{len(self.processes)}.log").open("w")
        self.output_files.append(output)
        process = subprocess.Popen(
            [sys.executable, str(RUNNER_PATH), role],
            stdout=output, stderr=subprocess.STDOUT, env=environment,
        )
        self.processes.append(process)
        return process

    def status(self, role="backend"):
        try:
            return json.loads((self.data / "run" / f"{role}.json").read_text())
        except (FileNotFoundError, json.JSONDecodeError):
            return {}

    def entries(self, name):
        path = self.data / name
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def wait_for(self, predicate, timeout=8):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            value = predicate()
            if value:
                return value
            time.sleep(0.025)
        logs = "\n".join(path.read_text() for path in sorted(self.root.glob("runner-*.log")))
        self.fail("condition timed out\n" + logs)

    def wait_state(self, state, role="backend"):
        return self.wait_for(lambda: self.status(role) if self.status(role).get("state") == state else None)

    def test_failure_stops_old_service_and_fix_rebuilds(self):
        self.start()
        old = self.wait_state("running")
        self.assertTrue(runner.healthy("backend", self.data))
        self.assertEqual(subprocess.run(
            [sys.executable, str(RUNNER_PATH), "health", "backend"],
            env={**os.environ, "EASYTIER_DATA_DIR": str(self.data)},
        ).returncode, 0)
        self.write("easytier-web/src/main.rs", "bad")
        failure = self.wait_state("failed")
        self.assertEqual(failure["exit_code"], 7)
        self.assertIsNone(runner.process_identity(old["process_pid"]))
        self.assertFalse(runner.healthy("backend", self.data))
        builds = len(self.entries("builds.jsonl"))
        time.sleep(0.3)
        self.assertEqual(len(self.entries("builds.jsonl")), builds, "same failing source must not loop")
        self.assertEqual([entry["version"] for entry in self.entries("starts.jsonl")], ["v1"])
        self.write("easytier-web/src/main.rs", "v2")
        self.wait_for(lambda: self.status().get("state") == "running"
                      and self.status().get("process_pid") != old["process_pid"])
        self.assertEqual(self.entries("starts.jsonl")[-1]["version"], "v2")

    def test_change_during_build_never_runs_superseded_output(self):
        self.write("easytier-web/src/main.rs", "slow-old")
        self.start()
        self.wait_for(lambda: (self.data / "building-slow").exists())
        self.write("easytier-web/src/main.rs", "latest")
        self.wait_state("running")
        self.assertEqual([entry["version"] for entry in self.entries("starts.jsonl")], ["latest"])
        self.assertEqual([entry["version"] for entry in self.entries("builds.jsonl")], ["slow-old", "latest"])

    def test_restart_rebuilds_and_does_not_reuse_stale_success(self):
        first = self.start()
        self.wait_state("running")
        first.terminate()
        self.assertEqual(first.wait(timeout=5), 0)
        self.assertTrue((self.data / "cache/target/debug/easytier-web").exists())
        self.write("easytier-web/src/main.rs", "bad")
        self.start()
        self.wait_state("failed")
        self.assertFalse(runner.healthy("backend", self.data))
        self.assertEqual([entry["version"] for entry in self.entries("starts.jsonl")], ["v1"])
        self.assertEqual(len(self.entries("builds.jsonl")), 2)

    def test_without_watch_build_failure_exits_nonzero(self):
        self.write("easytier-web/src/main.rs", "bad")
        process = self.start(EASYTIER_WATCH="false")
        self.assertEqual(process.wait(timeout=5), 7)
        self.assertEqual(self.status()["state"], "failed")
        self.assertEqual(self.entries("starts.jsonl"), [])

    def test_pause_stops_service_and_resume_builds_latest(self):
        self.start()
        old = self.wait_state("running")
        (self.data / "run/pause").touch()
        self.wait_state("stopped")
        self.assertIsNone(runner.process_identity(old["process_pid"]))
        self.write("easytier-web/src/main.rs", "paused-edit")
        time.sleep(0.2)
        self.assertEqual(len(self.entries("builds.jsonl")), 1)
        (self.data / "run/pause").unlink()
        self.wait_state("running")
        self.assertEqual(self.entries("starts.jsonl")[-1]["version"], "paused-edit")

    def test_runtime_exit_stays_failed_until_new_source(self):
        self.start()
        status = self.wait_state("running")
        os.kill(status["process_pid"], signal.SIGKILL)
        failed = self.wait_state("failed")
        self.assertEqual(failed["exit_code"], -signal.SIGKILL)
        time.sleep(0.2)
        self.assertEqual(len(self.entries("starts.jsonl")), 1)
        self.assertFalse(runner.healthy("backend", self.data))

    def test_startup_exit_stays_failed_until_source_is_fixed(self):
        self.write("easytier-web/src/main.rs", "start-fail")
        self.start()
        failure = self.wait_state("failed")
        self.assertEqual(failure["exit_code"], 11)
        self.assertFalse(runner.healthy("backend", self.data))
        time.sleep(0.2)
        self.assertEqual(len(self.entries("builds.jsonl")), 1)
        self.assertEqual(self.entries("starts.jsonl"), [])
        self.write("easytier-web/src/main.rs", "fixed-startup")
        self.wait_state("running")
        self.assertEqual(self.entries("starts.jsonl")[-1]["version"], "fixed-startup")

    def test_stop_during_build_cancels_builder_and_never_starts_service(self):
        self.write("easytier-web/src/main.rs", "slow-cancelled")
        process = self.start()
        self.wait_for(lambda: (self.data / "builder.pid").exists())
        builder_pid = int((self.data / "builder.pid").read_text())
        process.terminate()
        self.assertEqual(process.wait(timeout=5), 0)
        self.assertIsNone(runner.process_identity(builder_pid))
        self.assertEqual(self.entries("starts.jsonl"), [])
        self.assertEqual(self.status()["state"], "stopped")

    def test_stop_signal_cleans_entire_process_group(self):
        process = self.start(TEST_GRANDCHILD="1")
        status = self.wait_state("running")
        child_pid = int((self.data / "grandchild.pid").read_text())
        process.terminate()
        self.assertEqual(process.wait(timeout=5), 0)
        self.wait_for(lambda: runner.process_identity(child_pid) is None)
        self.assertIsNone(runner.process_identity(status["process_pid"]))
        self.assertEqual(self.status()["state"], "stopped")

    def test_source_filter_preserves_inputs_but_excludes_secrets_and_caches(self):
        for path in (".env", ".env.local", "data/secret", "node_modules/old.js", "target/old",
                     "easytier-web/frontend-lib/src/generated/stale.ts"):
            self.write(path, "private-or-cache")
        self.write("lifecycle-macros/src/lib.rs", "macro-v1")
        self.write("some-workspace-crate/Cargo.toml", "[package]\nname='fixture'\n")
        self.write("easytier-web/locales/en.yml", "hello: hello")
        self.write("easytier-web/src/obsolete.rs", "delete-me")
        self.start()
        old = self.wait_state("running")
        workspace = self.data / "workspaces/backend"
        self.assertTrue((workspace / "some-workspace-crate/Cargo.toml").exists())
        for path in (".env", ".env.local", "data/secret", "node_modules/old.js", "target/old",
                     "easytier-web/frontend-lib/src/generated/stale.ts"):
            self.assertFalse((workspace / path).exists(), path)
        self.write("lifecycle-macros/src/lib.rs", "macro-v2")
        (self.source / "easytier-web/src/obsolete.rs").unlink()
        self.wait_for(lambda: self.status().get("state") == "running"
                      and self.status().get("process_pid") != old["process_pid"])
        self.assertEqual((workspace / "lifecycle-macros/src/lib.rs").read_text(), "macro-v2")
        self.assertFalse((workspace / "easytier-web/src/obsolete.rs").exists())

    def test_frontend_source_sync_keeps_process_and_proto_regenerates(self):
        self.start("frontend")
        old = self.wait_state("running", "frontend")
        self.write("easytier-web/frontend/src/App.vue", "<template>second</template>")
        self.wait_for(lambda: self.status("frontend").get("source_revision") != old["source_revision"])
        self.assertEqual(self.status("frontend")["process_pid"], old["process_pid"])
        self.assertEqual([entry["step"] for entry in self.entries("pnpm.jsonl")], ["install", "codegen"])
        self.write("easytier-proto/proto/common.proto", "proto-v2")
        self.wait_for(lambda: self.status("frontend").get("state") == "running"
                      and self.status("frontend").get("process_pid") != old["process_pid"])
        self.assertEqual([entry["step"] for entry in self.entries("pnpm.jsonl")], ["install", "codegen", "codegen"])
        self.assertEqual((self.data / "workspaces/frontend/easytier-web/frontend-lib/src/generated/proto/common.ts").read_text(), "proto-v2")

    def test_frontend_dependency_failure_stops_service_and_retry_regenerates(self):
        self.start("frontend")
        old = self.wait_state("running", "frontend")
        self.write("easytier-web/frontend/package.json", '{"name":"easytier-frontend","version":"baddep"}')
        failure = self.wait_state("failed", "frontend")
        self.assertEqual(failure["exit_code"], 8)
        self.assertIsNone(runner.process_identity(old["process_pid"]))
        self.write("easytier-web/frontend/package.json", '{"name":"easytier-frontend","version":"2"}')
        self.wait_state("running", "frontend")
        self.assertEqual([entry["step"] for entry in self.entries("pnpm.jsonl")],
                         ["install", "codegen", "install", "install", "codegen"])

    def test_frontend_codegen_failure_removes_stale_generated_output(self):
        self.start("frontend")
        self.wait_state("running", "frontend")
        self.write("easytier-proto/proto/common.proto", "badproto")
        self.wait_state("failed", "frontend")
        self.assertFalse((self.data / "workspaces/frontend/easytier-web/frontend-lib/src/generated").exists())
        self.assertFalse(runner.healthy("frontend", self.data))
        self.write("easytier-proto/proto/common.proto", "fixedproto")
        self.wait_state("running", "frontend")
        self.assertEqual((self.data / "workspaces/frontend/easytier-web/frontend-lib/src/generated/proto/common.ts").read_text(), "fixedproto")


if __name__ == "__main__":
    unittest.main()
