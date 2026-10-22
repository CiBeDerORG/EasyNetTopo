"""Focused cron orchestration tests: fake crontab, real Bash, temporary Git checkout.

They do not start Docker or require a running cron daemon.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


REPO = Path(__file__).resolve().parents[2]


@unittest.skipUnless(os.geteuid() == 0 and shutil.which("git") and shutil.which("flock"),
                     "requires root, Git and flock to test the real cron entrypoint")
class ScheduleTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="easynettopo-cron-fixture-")
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name) / "folder with spaces" / "custom checkout"
        (self.root / "deploy").mkdir(parents=True)
        self.helper = self.root / "deploy" / "scheduled-deploy.sh"
        shutil.copy2(REPO / "deploy" / "scheduled-deploy.sh", self.helper)
        self.data = self.root / "data/run"
        self.crontab = Path(tmp.name) / "crontab.txt"
        self.crontab.write_text("20 6 * * * echo keep-unrelated-job\n")
        self.bin = Path(tmp.name) / "bin"
        self.bin.mkdir()
        cmd = self.bin / "crontab"
        cmd.write_text("#!/bin/sh\n"
                       "if [ \"$1\" = -l ]; then cat \"$TEST_CRONTAB\"; "
                       "elif [ \"$1\" = - ]; then cat > \"$TEST_CRONTAB\"; "
                       "else exit 2; fi\n")
        cmd.chmod(0o755)
        # Only schedule/check interplay is stubbed; no Docker or app starts.
        (self.root / "deploy.sh").write_text(
            "#!/bin/bash\n"
            "if [ \"$1\" = check ]; then\n"
            "  echo check >> \"$TEST_CHECK_LOG\"\n"
            "  if [ -n \"${TEST_WRITE_TREE:-}\" ]; then\n"
            "    printf '%s\\n' \"$TEST_WRITE_TREE\" > \"$(dirname \"$0\")/data/run/deployed-git-tree\"\n"
            "  fi\n"
            "  exit \"${TEST_CHECK_EXIT:-0}\"\n"
            "fi\nexit 9\n")
        subprocess.run(["git", "init", "-q", "-b", "main", str(self.root)], check=True)
        subprocess.run(["git", "-C", str(self.root), "config", "user.name", "Test"], check=True)
        subprocess.run(["git", "-C", str(self.root), "config", "user.email", "test@example.invalid"], check=True)
        (self.root / "README").write_text("content\n")
        subprocess.run(["git", "-C", str(self.root), "add", "README"], check=True)
        subprocess.run(["git", "-C", str(self.root), "commit", "-qm", "init"], check=True)
        self.checked = Path(tmp.name) / "check.log"
        self.env = dict(os.environ, PATH=f"${self.bin}:${os.environ['PATH']}",
                        TEST_CRONTAB=str(self.crontab), TEST_CHECK_LOG=str(self.checked),
                        SUDO_UID="0")

    def run_scheduler(self, *args, **env):
        return subprocess.run(["bash", str(self.helper), *args],
                              cwd=self.root, env=dict(self.env, **env),
                              capture_output=True, text=True, timeout=10)

    def test_start_status_and_stop_preserve_other_cron_jobs(self):
        start = self.run_scheduler("start", "2h")
        self.assertEqual(start.returncode, 0, start.stderr)
        cron = self.crontab.read_text()
        self.assertIn("20 6 * * * echo keep-unrelated-job", cron)
        self.assertIn("EASYNETTOPO_SCHEDULE:", cron)
        status = self.run_scheduler("status")
        self.assertIn("已启用", status.stdout)
        self.assertIn("120 分钟", status.stdout)
        self.assertIn("main", status.stdout)
        self.assertEqual((self.data / "scheduled.conf").read_text(), "120\n0\n")
        self.assertEqual((self.data / "scheduled.log").stat().st_mode & 0o777, 0o600)
        stopped = self.run_scheduler("stop")
        self.assertEqual(stopped.returncode, 0, stopped.stderr)
        self.assertEqual(self.crontab.read_text(), "20 6 * * * echo keep-unrelated-job\n")

    def test_unchanged_check_is_throttled(self):
        self.assertEqual(self.run_scheduler("start", "15m").returncode, 0)
        (self.data / "scheduled-last-check").write_text("0\n")
        first = self.run_scheduler("run")
        self.assertEqual(first.returncode, 0, first.stderr)
        second = self.run_scheduler("run")
        self.assertEqual(second.returncode, 0, second.stderr)
        self.assertEqual(self.checked.read_text().splitlines(), ["check"])
        self.assertIn("跳过部署", (self.data / "scheduled-last-result").read_text())

    def test_success_only_when_deployed_marker_advances(self):
        self.assertEqual(self.run_scheduler("start", "1m").returncode, 0)
        (self.data / "scheduled-last-check").write_text("0\n")
        success = self.run_scheduler("run", TEST_WRITE_TREE="test-tree-hash")
        self.assertEqual(success.returncode, 0, success.stderr)
        self.assertIn("部署成功", (self.data / "scheduled-last-result").read_text())

    def test_failed_check_keeps_last_success_marker(self):
        self.assertEqual(self.run_scheduler("start", "1m").returncode, 0)
        (self.data / "scheduled-last-check").write_text("0\n")
        marker = self.data / "deployed-git-tree"
        marker.write_text("old-tree\n")
        failed = self.run_scheduler("run", TEST_CHECK_EXIT="23")
        self.assertEqual(failed.returncode, 23)
        self.assertEqual(marker.read_text(), "old-tree\n")
        self.assertIn("退出码 23", (self.data / "scheduled-last-result").read_text())

    def test_lock_conflict_retry_does_not_consume_interval(self):
        self.assertEqual(self.run_scheduler("start", "1m").returncode, 0)
        (self.data / "scheduled-last-check").write_text("0\n")
        skipped = self.run_scheduler("run", TEST_CHECK_EXIT="75")
        self.assertEqual(skipped.returncode, 0)
        self.assertEqual((self.data / "scheduled-last-check").read_text(), "0\n")
        self.assertIn("下一分钟重试", (self.data / "scheduled-last-result").read_text())

    def test_invalid_intervals_do_not_overwrite_cron(self):
        for interval in ("0", "10s", "10081m", "-1", "1q"):
            with self.subTest(interval=interval):
                result = self.run_scheduler("start", interval)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.crontab.read_text(), "20 6 * * * echo keep-unrelated-job\n")


if __name__ == "__main__":
    unittest.main()
