#!/usr/bin/env python3
"""Own one development service, its source snapshot and all child processes.

The source checkout is read-only. Builds and generated files live in a separate
workspace below /data. A failed or superseded build never starts an old binary.
"""

from __future__ import annotations

import argparse
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import signal
import shutil
import socket
import stat
import subprocess
import sys
import tempfile
import threading
import time


EXCLUDED_DIRS = {
    ".git", "data", "node_modules", "target", "dist", "generated",
    "__pycache__", ".vite", ".cache", ".pnpm-store",
}
FRONTEND_DIRS = {"frontend", "frontend-lib", "config-generator"}
DEPENDENCY_FILES = {"package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml", ".npmrc"}
OPTIONAL_ENV = {
    "ET_WEB_PUBLIC_ORIGIN", "ET_API_HOST", "ET_WEB_SERVER_PORT",
    "ET_WEBHOOK_URL", "ET_WEBHOOK_SECRET", "ET_INTERNAL_AUTH_TOKEN",
    "ET_WEB_INSTANCE_ID", "ET_WEB_INSTANCE_API_BASE_URL", "ET_GATEWAY_PEER_URL",
    "ET_GEOIP_DB", "ET_WEB_CONSOLE_LOG_LEVEL", "ET_WEB_FILE_LOG_LEVEL",
    "ET_WEB_FILE_LOG_DIR", "OIDC_CLIENT_SECRET",
}
Snapshot = dict[str, tuple[int, int, int, int, str]]


class SourceChanged(Exception):
    """The in-flight operation no longer represents the current source."""


class Paused(Exception):
    pass


class Stopped(Exception):
    pass


class StepFailed(Exception):
    def __init__(self, stage: str, exit_code: int):
        super().__init__(stage)
        self.stage = stage
        self.exit_code = exit_code


def excluded_file(name: str) -> bool:
    return (
        name == ".env" or name.startswith(".env.") or name == ".envrc"
        or name.endswith((".pyc", ".tsbuildinfo"))
    )


def watches(role: str, path: str) -> bool:
    """Include build scripts, local macros, schemas and embedded crate assets."""
    parts = Path(path).parts
    name = parts[-1]
    if role == "backend":
        if name in {"Cargo.toml", "Cargo.lock", "rust-toolchain", "rust-toolchain.toml"}:
            return True
        if parts[0] == ".cargo" or name.endswith((".rs", ".proto")):
            return True
        if parts[0] in {"easytier", "easytier-core", "easytier-proto"}:
            return True
        return parts[0] == "easytier-web" and (len(parts) < 2 or parts[1] not in FRONTEND_DIRS)
    return (
        name in DEPENDENCY_FILES
        or name.endswith(".proto")
        or path == "deploy/frontend.mjs"
        or (parts[0] == "easytier-web" and len(parts) > 1
            and parts[1] in {"frontend", "frontend-lib"})
    )


def source_snapshot(source: Path, role: str) -> Snapshot:
    if not source.is_dir():
        raise FileNotFoundError("源码目录不存在")
    result: Snapshot = {}

    def on_error(error: OSError) -> None:
        if not isinstance(error, FileNotFoundError):
            raise error

    for directory, directories, filenames in os.walk(source, onerror=on_error):
        directories[:] = sorted(name for name in directories if name not in EXCLUDED_DIRS)
        # Do not follow symlinks; their targets may be outside the source tree.
        symlinks = [name for name in directories if (Path(directory) / name).is_symlink()]
        directories[:] = [name for name in directories if name not in symlinks]
        for name in sorted(filenames + symlinks):
            if excluded_file(name):
                continue
            filename = Path(directory) / name
            relative = filename.relative_to(source).as_posix()
            if not watches(role, relative):
                continue
            try:
                info = filename.lstat()
                link = os.readlink(filename) if stat.S_ISLNK(info.st_mode) else ""
            except FileNotFoundError:
                # An editor/git can replace a file while the tree is scanned.
                continue
            result[relative] = (info.st_mtime_ns, info.st_ctime_ns, info.st_size, info.st_mode, link)
    return result


def revision(snapshot: Snapshot) -> str:
    return hashlib.sha256(json.dumps(snapshot, sort_keys=True).encode()).hexdigest()


def process_identity(pid: int) -> str | None:
    """Linux start ticks distinguish a live process from a reused/stale PID."""
    try:
        fields = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()
        if fields[0] in {"Z", "X"}:
            return None
        return fields[19]
    except (OSError, IndexError, ValueError):
        return None


def healthy(role: str, data: Path = Path("/data")) -> bool:
    try:
        status = json.loads((data / "run" / f"{role}.json").read_text())
        if status["state"] != "running" or (data / "run" / "pause").exists():
            return False
        return all(
            status.get(f"{prefix}_start") is not None
            and process_identity(int(status[f"{prefix}_pid"])) == status[f"{prefix}_start"]
            for prefix in ("runner", "process")
        )
    except (OSError, ValueError, KeyError, TypeError):
        return False


def terminate(process: subprocess.Popen | None) -> None:
    if process is None:
        return
    # Every command owns a session, including cargo/pnpm and their descendants.
    with contextlib.suppress(ProcessLookupError):
        os.killpg(process.pid, signal.SIGTERM)
    try:
        process.wait(timeout=3)
    except subprocess.TimeoutExpired:
        pass
    # A parent may exit before a descendant that ignores TERM. Reap that group too.
    with contextlib.suppress(ProcessLookupError):
        os.killpg(process.pid, signal.SIGKILL)
    process.wait()


class Runner:
    def __init__(
        self, role: str, source: Path = Path("/source"), data: Path = Path("/data"),
        environment: dict[str, str] | None = None,
    ):
        self.role = role
        self.source = source.resolve()
        self.data = data.resolve()
        self.workspace = self.data / "workspaces" / role
        self.run_dir = self.data / "run"
        self.pause_file = self.run_dir / "pause"
        self.env = dict(os.environ if environment is None else environment)
        self.watch = self.env.get("EASYTIER_WATCH", "true").lower()
        if self.watch not in {"true", "false"}:
            raise ValueError("EASYTIER_WATCH 必须为 true 或 false")
        self.watch = self.watch == "true"
        self.poll_interval = float(self.env.get("EASYTIER_POLL_INTERVAL", "1"))
        self.debounce = float(self.env.get("EASYTIER_DEBOUNCE", "1"))
        if not 0 < self.poll_interval < float("inf") or not 0 <= self.debounce < float("inf"):
            raise ValueError("轮询间隔必须大于 0，防抖时间必须大于等于 0")
        self.profile = self.env.get("CARGO_BUILD_PROFILE", "dev")
        if self.profile not in {"dev", "release"}:
            raise ValueError("CARGO_BUILD_PROFILE 必须为 dev 或 release")
        self.env.setdefault("CARGO_BUILD_JOBS", "2")
        self.stop_event = threading.Event()
        self.child: subprocess.Popen | None = None
        self.step: subprocess.Popen | None = None
        self.state = "starting"
        self.applied: Snapshot | None = None
        self.installed: Snapshot | None = None
        self.generated: Snapshot | None = None
        self.lock_file = None

    def prepare_directories(self) -> None:
        paths = {
            "CARGO_HOME": self.data / "cache" / "cargo",
            "CARGO_TARGET_DIR": self.data / "cache" / "target",
            "npm_config_cache": self.data / "cache" / "npm",
            "XDG_CACHE_HOME": self.data / "cache" / self.role,
            "XDG_CONFIG_HOME": self.data / "config" / self.role,
            "XDG_STATE_HOME": self.data / "state" / self.role,
            "XDG_DATA_HOME": self.data / "local" / self.role,
            "TMPDIR": self.data / "tmp" / self.role,
        }
        for key, path in paths.items():
            path.mkdir(parents=True, exist_ok=True)
            self.env[key] = str(path)
        for path in (self.workspace, self.run_dir, self.data / "web", self.data / "cache" / "pnpm"):
            path.mkdir(parents=True, exist_ok=True)
        self.env["EASYTIER_SOURCE_DIR"] = str(self.source)
        self.env["EASYTIER_DATA_DIR"] = str(self.data)
        self.env.setdefault("ET_WEB_DB", str(self.data / "web" / "et.db"))
        self.env["PYTHONDONTWRITEBYTECODE"] = "1"
        for key in OPTIONAL_ENV:
            if self.env.get(key) == "":
                del self.env[key]

    def publish(self, state: str, message: str, snapshot: Snapshot | None = None, exit_code=None) -> None:
        self.state = state
        payload = {
            "role": self.role, "state": state, "message": message,
            "updated_at": time.time(), "runner_pid": os.getpid(),
            "runner_start": process_identity(os.getpid()),
            "process_pid": self.child.pid if self.child else None,
            "process_start": process_identity(self.child.pid) if self.child else None,
            "source_revision": revision(snapshot) if snapshot is not None else None,
        }
        if exit_code is not None:
            payload["exit_code"] = exit_code
        build_env = {key: self.env.get(key, "") for key in (
            "RUSTUP_TOOLCHAIN", "RUSTC", "RUSTFLAGS", "CARGO_ENCODED_RUSTFLAGS",
            "CARGO_BUILD_PROFILE", "CARGO_BUILD_JOBS",
        )}
        payload["build_environment_revision"] = hashlib.sha256(
            json.dumps(build_env, sort_keys=True).encode()
        ).hexdigest()
        with tempfile.NamedTemporaryFile("w", dir=self.run_dir, prefix=f".{self.role}.", delete=False) as output:
            json.dump(payload, output, ensure_ascii=False)
            temporary = output.name
        os.replace(temporary, self.run_dir / f"{self.role}.json")
        print(f"[{self.role}] {state}: {message}", flush=True)

    def check_control(self) -> None:
        if self.stop_event.is_set():
            raise Stopped()
        if self.pause_file.exists():
            raise Paused()

    def check_snapshot(self, expected: Snapshot) -> None:
        self.check_control()
        if source_snapshot(self.source, self.role) != expected:
            raise SourceChanged()

    def stable_snapshot(self, current: Snapshot) -> Snapshot:
        changed_at = time.monotonic()
        while True:
            self.check_control()
            following = source_snapshot(self.source, self.role)
            if following != current:
                current, changed_at = following, time.monotonic()
            remaining = self.debounce - (time.monotonic() - changed_at)
            if remaining <= 0:
                return current
            self.stop_event.wait(min(self.poll_interval, remaining))

    def command(self, arguments: list[str], stage: str, expected: Snapshot) -> None:
        self.check_snapshot(expected)
        print(f"[{self.role}] {stage}", flush=True)
        try:
            self.step = subprocess.Popen(arguments, cwd=self.workspace, env=self.env, start_new_session=True)
        except OSError:
            raise StepFailed(f"{stage}：无法启动命令", 127) from None
        try:
            next_scan = time.monotonic() + self.poll_interval
            while self.step.poll() is None:
                self.check_control()
                if time.monotonic() >= next_scan:
                    self.check_snapshot(expected)
                    next_scan = time.monotonic() + self.poll_interval
                self.stop_event.wait(min(self.poll_interval, 0.2))
            code = self.step.returncode
            self.check_snapshot(expected)
            if code != 0:
                raise StepFailed(stage, code)
        finally:
            terminate(self.step)
            self.step = None

    def synchronize(self, expected: Snapshot) -> None:
        # Match contents/modes, not host ownership: containers run as LOCAL_UID.
        arguments = ["rsync", "-rltp", "--checksum", "--delete", "--safe-links"]
        arguments += [f"--exclude={name}/" for name in sorted(EXCLUDED_DIRS)]
        arguments += ["--exclude=.env", "--exclude=.env.*", "--exclude=.envrc",
                      "--exclude=*.pyc", "--exclude=*.tsbuildinfo"]
        arguments += [str(self.source) + "/", str(self.workspace) + "/"]
        try:
            self.command(arguments, "同步源码", expected)
        except StepFailed as error:
            # Editors/git may remove even an unwatched file while rsync reads it.
            if error.exit_code == 24:
                raise SourceChanged() from None
            raise

    def stop_child(self) -> None:
        terminate(self.child)
        self.child = None

    def start_child(self, command: list[str], expected: Snapshot) -> None:
        self.check_snapshot(expected)
        try:
            self.child = subprocess.Popen(command, cwd=self.workspace, env=self.env, start_new_session=True)
        except OSError:
            raise StepFailed("服务启动失败", 127) from None
        if self.role == "backend":
            host = self.env.get("ET_API_SERVER_ADDR", "0.0.0.0")
            host = {"0.0.0.0": "127.0.0.1", "::": "::1"}.get(host, host)
            port = int(self.env.get("ET_API_SERVER_PORT", "11211"))
        else:
            host, port = "127.0.0.1", int(self.env.get("WEB_PORT", "11210"))
        try:
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                self.check_snapshot(expected)
                if self.child.poll() is not None:
                    raise StepFailed("服务启动后退出", self.child.returncode)
                try:
                    with socket.create_connection((host, port), timeout=0.2):
                        pass
                    # Detect an immediate startup error before publishing success.
                    self.stop_event.wait(0.1)
                    self.check_snapshot(expected)
                    if self.child.poll() is None:
                        self.applied = expected
                        self.publish("running", "服务已启动", expected)
                        return
                except OSError:
                    pass
                self.stop_event.wait(min(self.poll_interval, 0.2))
            raise StepFailed("服务端口就绪检查超时", 1)
        except BaseException:
            self.stop_child()
            raise

    def reconcile(self, expected: Snapshot) -> None:
        previous = self.applied or {}
        changed = {path for path in previous.keys() | expected.keys() if previous.get(path) != expected.get(path)}
        dependencies = {path: stamp for path, stamp in expected.items() if Path(path).name in DEPENDENCY_FILES}
        protocols = {path: stamp for path, stamp in expected.items()
                     if path.endswith(".proto") or path.endswith("/scripts/codegen-proto.mjs")}
        configuration_changed = any(
            path == "deploy/frontend.mjs" or Path(path).name.startswith(("vite.config.", "postcss.config.", "tailwind.config."))
            for path in changed
        )
        restarting = (
            self.role == "backend" or self.child is None or dependencies != self.installed
            or protocols != self.generated or configuration_changed
        )
        if restarting:
            self.stop_child()
            self.publish("building", "准备当前源码", expected)
        self.synchronize(expected)
        if self.role == "backend":
            arguments = ["cargo", "build", "--locked", "-p", "easytier-web", "--bin", "easytier-web"]
            if self.profile == "release":
                arguments.append("--release")
            self.command(arguments, "Cargo 增量编译", expected)
            executable = Path(self.env["CARGO_TARGET_DIR"]) / ("release" if self.profile == "release" else "debug") / "easytier-web"
            self.start_child([str(executable)], expected)
        elif restarting:
            dependencies_changed = dependencies != self.installed
            if dependencies_changed:
                self.command([
                    "pnpm", "--filter", "easytier-frontend...", "install", "--frozen-lockfile",
                    "--store-dir", str(self.data / "cache" / "pnpm"),
                ], "安装前端锁定依赖", expected)
                self.installed = dependencies
            if dependencies_changed or protocols != self.generated:
                self.generated = None
                with contextlib.suppress(FileNotFoundError):
                    shutil.rmtree(self.workspace / "easytier-web/frontend-lib/src/generated")
                self.command(["pnpm", "--filter", "easytier-frontend-lib", "codegen:proto"], "生成前端协议类型", expected)
                self.generated = protocols
            self.start_child(["node", "deploy/frontend.mjs"], expected)
        else:
            if self.child.poll() is not None:
                raise StepFailed("前端进程已退出", self.child.returncode)
            self.applied = expected
            self.publish("running", "源码已同步，等待 Vite 热更新", expected)

    def failed(self, error: StepFailed, snapshot: Snapshot | None) -> None:
        self.stop_child()
        self.publish("failed", f"{error.stage}（退出码 {error.exit_code}）；服务保持停止", snapshot, error.exit_code)

    def run(self) -> int:
        self.prepare_directories()
        self.lock_file = (self.run_dir / f"{self.role}.lock").open("a")
        try:
            fcntl.flock(self.lock_file, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print(f"[{self.role}] 已有运行器使用该数据目录", file=sys.stderr)
            self.lock_file.close()
            return 2
        for signum in (signal.SIGINT, signal.SIGTERM):
            signal.signal(signum, lambda *_: self.stop_event.set())
        # Never inherit a previous container's successful status or build result.
        self.publish("starting", "检查源码和构建环境")
        last_attempt: Snapshot | None = None
        try:
            while not self.stop_event.is_set():
                try:
                    self.check_control()
                    current = source_snapshot(self.source, self.role) if self.watch or last_attempt is None else last_attempt
                    if self.child is not None and self.child.poll() is not None:
                        self.failed(StepFailed("服务进程意外退出", self.child.returncode), self.applied)
                        if not self.watch:
                            return 1
                    if current != last_attempt:
                        if self.role == "backend":
                            self.stop_child()
                        current = self.stable_snapshot(current)
                        try:
                            self.reconcile(current)
                        except StepFailed as error:
                            self.failed(error, current)
                            if not self.watch:
                                return error.exit_code if 0 < error.exit_code < 126 else 1
                        last_attempt = current
                    self.stop_event.wait(self.poll_interval)
                except SourceChanged:
                    if self.role == "backend":
                        self.stop_child()
                    # Do not publish/run the superseded output, even with WATCH=false.
                    last_attempt = None
                except Paused:
                    self.stop_child()
                    if self.state != "stopped":
                        self.publish("stopped", "部署已暂停")
                    self.applied = None
                    last_attempt = None
                    self.stop_event.wait(self.poll_interval)
                except OSError as error:
                    self.failed(StepFailed(f"源码或工作目录操作失败：{error.strerror or type(error).__name__}", 1), None)
                    return 1
            return 0
        except Stopped:
            return 0
        finally:
            terminate(self.step)
            self.stop_child()
            if self.state != "failed":
                self.publish("stopped", "运行器已停止")
            self.lock_file.close()


def main() -> int:
    parser = argparse.ArgumentParser(description="EasyTier Docker 开发运行器")
    parser.add_argument("action", choices=("backend", "frontend", "health"))
    parser.add_argument("service", nargs="?", choices=("backend", "frontend"))
    arguments = parser.parse_args()
    source = Path(os.environ.get("EASYTIER_SOURCE_DIR") or "/source")
    data = Path(os.environ.get("EASYTIER_DATA_DIR") or "/data")
    if arguments.action == "health":
        if not arguments.service:
            parser.error("health 需要 backend 或 frontend")
        return 0 if healthy(arguments.service, data) else 1
    if arguments.service:
        parser.error("启动服务时不需要第二个参数")
    try:
        return Runner(arguments.action, source, data).run()
    except (ValueError, OSError) as error:
        print(f"运行器配置或目录错误：{error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
