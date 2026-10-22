"""Offline tests of the authenticated runtime proxy lookup; fake Docker only."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "deploy/runtime-proxy.sh"


class RuntimeProxyTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="runtime-proxy-")
        self.addCleanup(tmp.cleanup)
        self.bin = Path(tmp.name)
        docker = self.bin / "docker"
        docker.write_text(
            "#!/usr/bin/env python3\n"
            "import os, sys\n"
            "args = sys.argv[1:]\n"
            "if args[:2] in (['network', 'inspect'], ['container', 'inspect']):\n"
            "    sys.exit(0)\n"
            "if args[0] == 'inspect':\n"
            "    template = args[args.index('-f') + 1]\n"
            "    if '.Config.Env' in template:\n"
            "        print('HTTPS_PROXY=' + os.environ.get('MOCK_PROXY', 'http://fake:secret@mihomo:7890'))\n"
            "    elif '.State.Running' in template:\n"
            "        print('false' if os.environ.get('MOCK_MIHOMO_DOWN') else 'true')\n"
            "    elif 'proxy-net' in template:\n"
            "        print('yes')\n"
            "    else:\n"
            "        sys.exit(3)\n"
            "    sys.exit(0)\n"
            "sys.exit(2)\n"
        )
        docker.chmod(0o755)

    def run_helper(self, **overrides):
        env = dict(os.environ)
        env.update(PATH=f"{self.bin}:{os.environ['PATH']}",
                   BUILDX_BUILDER="easyadb-proxy", HELPER=str(HELPER))
        env.update(overrides)
        return subprocess.run(
            ["bash", "-c", 'source "$HELPER"; '
             'test "$EASYTIER_RUNTIME_PROXY" = "http://fake:secret@mihomo:7890" '
             '&& echo OK'],
            env=env, text=True, capture_output=True, timeout=10,
        )

    def test_proxy_from_existing_builder_is_exported_without_echoing_password(self):
        out = self.run_helper()
        self.assertEqual(out.returncode, 0, out.stderr)
        self.assertIn("OK", out.stdout)
        self.assertNotIn("fake:secret", out.stdout + out.stderr)

    def test_stopped_mihomo_fails_before_recreating_containers(self):
        out = self.run_helper(MOCK_MIHOMO_DOWN="1")
        self.assertNotEqual(out.returncode, 0)
        self.assertIn("mihomo 容器未运行", out.stderr)
        self.assertNotIn("fake:secret", out.stdout + out.stderr)

    def test_unexpected_proxy_is_rejected_without_disclosing_credentials(self):
        out = self.run_helper(MOCK_PROXY="http://bad:secret@example.invalid:7890")
        self.assertNotEqual(out.returncode, 0)
        self.assertIn("mihomo:7890", out.stderr)
        self.assertNotIn("bad:secret", out.stdout + out.stderr)


if __name__ == "__main__":
    unittest.main()
