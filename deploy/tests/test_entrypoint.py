"""Exercise the real Bash entrypoint and real Git using a fake Docker command.

These tests validate orchestration and failure boundaries, not Docker builds.
All credentials here are deliberately fictional, and all files are temporary.
"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


REPO = Path(__file__).resolve().parents[2]
REAL_GIT = shutil.which("git")
DOCKER_STUB = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
log = pathlib.Path(os.environ['TEST_COMMAND_LOG'])
with log.open('a') as out:
    out.write(json.dumps(args) + '\n')
if args[0] in ('info', 'inspect'):
    if args[0] == 'inspect': print('running healthy')
    sys.exit(0)
if args[0] != 'compose': sys.exit(2)
commands = {'version','stop','down','build','up','ps','exec','logs'}
command = next((arg for arg in args[1:] if arg in commands), None)
if command == 'build':
    pathlib.Path(os.environ['TEST_CAPTURE']).write_text(json.dumps({
        key: os.environ.get(key) for key in (
            'ET_INTERNAL_AUTH_TOKEN', 'ET_WEBHOOK_SECRET', 'GIT_BRANCH',
            'EASYTIER_SOURCE_DIR', 'EASYTIER_DATA_DIR', 'WEB_PORT',
            'CONFIG_TRANSPORT', 'ET_CONFIG_SERVER_PROTOCOL')
    }))
    sys.exit(int(os.environ.get('TEST_FAIL_BUILD', '0')))
if command == 'up' and os.environ.get('TEST_FAIL_ROLE'):
    data = pathlib.Path(os.environ['EASYTIER_DATA_DIR']) / 'run'
    role = os.environ['TEST_FAIL_ROLE']
    (data / (role + '.json')).write_text(json.dumps({'state': 'failed', 'exit_code': 17}))
if command == 'ps' and '--quiet' in args: print('test-' + args[-1])
if command == 'exec': sys.exit(int(os.environ.get('TEST_NOT_READY', '0')))
sys.exit(0)
'''

GIT_STUB = r'''#!/usr/bin/env python3
import os, sys
args = sys.argv[1:]
# Only the network operation uses the local bare remote. Origin inspection
# still validates the actual CiBeDerORG/EasyNetTopo URL configured in the repo.
if any(arg in ('pull', 'fetch') for arg in args):
    args = ['-c', 'url.' + os.environ['TEST_REMOTE'] + '.insteadOf=https://github.com/CiBeDerORG/EasyNetTopo.git'] + args
os.execv(os.environ['TEST_REAL_GIT'], [os.environ['TEST_REAL_GIT'], *args])
'''


@unittest.skipUnless(REAL_GIT and shutil.which("bash") and shutil.which("flock"), "requires Bash, Git and flock")
class EntrypointTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="easytier-entry-")
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.repo = self.base / "modding with spaces" / "custom checkout"
        (self.repo / "deploy").mkdir(parents=True)
        shutil.copy2(REPO / "deploy.sh", self.repo / "deploy.sh")
        for name in (".env.example", "compose.yaml", "Dockerfile"):
            shutil.copy2(REPO / "deploy" / name, self.repo / "deploy" / name)
        (self.repo / ".gitignore").write_text("/.env\n/data/\n")
        (self.repo / "tracked.txt").write_text("initial\n")
        self.git("init", "-b", "dev")
        self.git("config", "user.name", "Deployment test")
        self.git("config", "user.email", "deployment-test@example.invalid")
        self.git("add", ".")
        self.git("commit", "-m", "fixture")
        self.git("remote", "add", "origin", "https://github.com/CiBeDerORG/EasyNetTopo.git")
        self.remote = self.base / "remote.git"
        subprocess.run([REAL_GIT, "init", "--bare", str(self.remote)], check=True, capture_output=True)
        self.git("push", str(self.remote), "HEAD:refs/heads/dev")
        self.bin = self.base / "bin"
        self.bin.mkdir()
        for name, source in (("docker", DOCKER_STUB), ("git", GIT_STUB)):
            script = self.bin / name
            script.write_text(source)
            script.chmod(0o755)
        self.log = self.base / "commands.jsonl"
        self.capture = self.base / "environment.json"
        self.env = dict(os.environ, PATH=f"{self.bin}:{os.environ['PATH']}",
                        TEST_COMMAND_LOG=str(self.log), TEST_CAPTURE=str(self.capture),
                        TEST_REAL_GIT=REAL_GIT, TEST_REMOTE=str(self.remote))

    def git(self, *args):
        return subprocess.run([REAL_GIT, "-C", str(self.repo), *args], check=True,
                              text=True, capture_output=True).stdout.strip()

    def run_entry(self, *args, **extra):
        return subprocess.run(["bash", str(self.repo / "deploy.sh"), *args],
                              cwd=self.base, env=dict(self.env, **extra),
                              text=True, capture_output=True, timeout=20)

    def write_env(self, content):
        (self.repo / ".env").write_text(content)

    def commands(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []

    def actions(self):
        names = {"stop", "down", "build", "up", "ps", "exec", "logs"}
        return [next(arg for arg in cmd if arg in names) for cmd in self.commands() if any(arg in names for arg in cmd)]

    def test_first_init_from_parent_creates_template_only_in_repo(self):
        result = self.run_entry("init")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.repo / ".env").read_bytes(), (self.repo / "deploy/.env.example").read_bytes())
        self.assertTrue((self.repo / "data/run").is_dir())
        self.assertFalse((self.base / ".env").exists())
        self.assertEqual((self.repo / ".env").stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.commands(), [])

    def test_existing_env_is_not_overwritten(self):
        value = "WEB_PORT=12345\nET_INTERNAL_AUTH_TOKEN='fictional-only'\n"
        self.write_env(value)
        result = self.run_entry("init")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.repo / ".env").read_text(), value)

    def test_secrets_are_literal_not_commands_or_compose_interpolation(self):
        sentinel = self.base / "must-not-exist"
        token = f"fictional $HOME ${{WEB_PORT}} $(touch {sentinel}) `touch {sentinel}` # = tail"
        self.write_env(f"ET_INTERNAL_AUTH_TOKEN='{token}'\nET_WEBHOOK_SECRET=fictional$abc#=42\nWEB_PORT=12345\n")
        result = self.run_entry("up")
        self.assertEqual(result.returncode, 0, result.stderr)
        capture = json.loads(self.capture.read_text())
        self.assertEqual(capture["ET_INTERNAL_AUTH_TOKEN"], token)
        self.assertEqual(capture["ET_WEBHOOK_SECRET"], "fictional$abc#=42")
        self.assertEqual(capture["EASYTIER_SOURCE_DIR"], str(self.repo))
        self.assertEqual(capture["EASYTIER_DATA_DIR"], str(self.repo / "data"))
        self.assertFalse(sentinel.exists())
        self.assertNotIn(token, result.stdout + result.stderr)
        self.assertLess(self.actions().index("stop"), self.actions().index("build"))
        self.assertLess(self.actions().index("build"), self.actions().index("up"))

    def test_invalid_settings_fail_without_printing_secret(self):
        self.write_env("ET_INTERNAL_AUTH_TOKEN='fictional-secret'\nWEB_PORT=65536\n")
        result = self.run_entry("up")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("WEB_PORT", result.stderr)
        self.assertNotIn("fictional-secret", result.stderr)
        self.assertEqual(self.commands(), [])

    def test_unclosed_quote_and_unknown_variable_are_rejected(self):
        for value in ("ET_INTERNAL_AUTH_TOKEN='fictional-unclosed\n", "PATH=fictional\n"):
            with self.subTest(value=value.split("=", 1)[0]):
                self.write_env(value)
                result = self.run_entry("init")
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("fictional", result.stderr)

    def test_failed_image_build_never_starts_old_application(self):
        result = self.run_entry("up", TEST_FAIL_BUILD="23")
        self.assertEqual(result.returncode, 23, result.stderr)
        self.assertEqual(self.actions(), ["stop", "build"])
        self.assertFalse((self.repo / "data/run/pause").exists())

    def test_failed_source_build_is_a_nonzero_deploy_result(self):
        result = self.run_entry("up", TEST_FAIL_ROLE="backend")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("backend", result.stderr)
        self.assertEqual(self.actions().count("up"), 1)

    def test_stop_preserves_application_data(self):
        (self.repo / "data/web").mkdir(parents=True)
        db = self.repo / "data/web/et.db"
        db.write_bytes(b"fixture-database")
        result = self.run_entry("stop")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(db.read_bytes(), b"fixture-database")
        self.assertIn("down", self.actions())

    def test_ws_config_server_publishes_tcp_transport(self):
        self.write_env("ET_CONFIG_SERVER_PROTOCOL=ws\n")
        result = self.run_entry("up")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(self.capture.read_text())["CONFIG_TRANSPORT"], "tcp")

    def test_selected_local_branch_is_used_and_lock_survives_reexec(self):
        self.git("branch", "another")
        self.write_env("GIT_BRANCH=another\n")
        result = self.run_entry("up")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git("branch", "--show-current"), "another")
        self.assertEqual(self.actions().count("build"), 1)

    def test_switch_refuses_to_discard_local_changes(self):
        self.git("branch", "another")
        (self.repo / "tracked.txt").write_text("local edit\n")
        self.write_env("GIT_BRANCH=another\n")
        result = self.run_entry("up")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("branch", "--show-current"), "dev")
        self.assertEqual((self.repo / "tracked.txt").read_text(), "local edit\n")
        self.assertEqual(self.actions(), [])

    def test_update_only_fast_forwards(self):
        old = self.git("rev-parse", "HEAD")
        (self.repo / "tracked.txt").write_text("remote edit\n")
        self.git("commit", "-am", "remote change")
        newest = self.git("rev-parse", "HEAD")
        self.git("push", str(self.remote), "HEAD:refs/heads/dev")
        # Fixture-only reset establishes an outdated clean checkout; the
        # deployment entrypoint itself must never use reset.
        self.git("reset", "--hard", old)
        result = self.run_entry("update")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git("rev-parse", "HEAD"), newest)
        self.assertEqual(self.actions().count("build"), 1)

    def test_diverged_update_does_not_reset_or_start_old_version(self):
        old = self.git("rev-parse", "HEAD")
        (self.repo / "remote.txt").write_text("remote\n")
        self.git("add", "remote.txt")
        self.git("commit", "-m", "remote diverges")
        self.git("push", str(self.remote), "HEAD:refs/heads/dev")
        self.git("reset", "--hard", old)
        (self.repo / "local.txt").write_text("local\n")
        self.git("add", "local.txt")
        self.git("commit", "-m", "local diverges")
        local_head = self.git("rev-parse", "HEAD")
        result = self.run_entry("update")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("rev-parse", "HEAD"), local_head)
        self.assertEqual(self.actions(), ["stop"])

    def test_another_repository_is_rejected(self):
        self.git("remote", "set-url", "origin", "https://github.com/Example/Another.git")
        result = self.run_entry("up")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.actions(), [])

    def test_same_repository_path_on_another_host_is_rejected(self):
        self.git("remote", "set-url", "origin", "https://example.invalid/CiBeDerORG/EasyNetTopo.git")
        result = self.run_entry("up")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.actions(), [])


if __name__ == "__main__":
    unittest.main()
