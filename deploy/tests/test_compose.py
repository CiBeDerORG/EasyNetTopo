"""Validate the configuration with the real Compose parser, without a daemon.

Set COMPOSE_BIN to an official standalone Docker Compose v2 executable.
"""

import json
import os
from pathlib import Path
import subprocess
import unittest

REPO = Path(__file__).resolve().parents[2]
COMPOSE_BIN = os.environ.get("COMPOSE_BIN")


@unittest.skipUnless(COMPOSE_BIN, "set COMPOSE_BIN to test with the real Compose parser")
class ComposeConfigTests(unittest.TestCase):
    def config(self, *, runtime_proxy=False, **overrides):
        env = dict(os.environ)
        for line in (REPO / "deploy/.env.example").read_text().splitlines():
            if line and not line.startswith("#"):
                key, value = line.split("=", 1)
                env[key] = value
        env.update(EASYTIER_SOURCE_DIR=str(REPO), EASYTIER_DATA_DIR=str(REPO / "data"),
                   LOCAL_UID=str(os.getuid()), LOCAL_GID=str(os.getgid()),
                   CONFIG_TRANSPORT="udp")
        env.update(overrides)
        args = [COMPOSE_BIN, "--project-directory", str(REPO), "--env-file", "/dev/null",
                "--file", str(REPO / "deploy/compose.yaml")]
        if runtime_proxy:
            args.extend(["--file", str(REPO / "deploy/compose.proxy.yaml")])
        args.extend(["config", "--format", "json"])
        result = subprocess.run(args, env=env, text=True, capture_output=True, check=True)
        return json.loads(result.stdout)

    def test_paths_secret_isolation_and_failure_policy(self):
        token = "fictional$HOME ${WEB_PORT} #=token"
        config = self.config(ET_INTERNAL_AUTH_TOKEN=token)
        self.assertEqual(set(config["services"]), {"backend", "frontend"})
        backend, frontend = config["services"]["backend"], config["services"]["frontend"]
        # Compose config's serializer escapes dollars AFTER interpolation.
        # docker/compose v2.39.4 cmd/compose/config.go::escapeDollarSign.
        self.assertEqual(backend["environment"]["ET_INTERNAL_AUTH_TOKEN"], token.replace("$", "$$"))
        self.assertNotIn("ET_INTERNAL_AUTH_TOKEN", frontend["environment"])
        self.assertEqual(backend["build"]["context"], str(REPO / "deploy"))
        self.assertEqual(backend["environment"]["ET_WEB_DB"], "/data/web/et.db")
        self.assertEqual(backend["user"], f"{os.getuid()}:{os.getgid()}")
        self.assertEqual(backend["restart"], "no")
        mounts = {entry["target"]: entry for entry in backend["volumes"]}
        self.assertEqual(mounts["/data"]["source"], str(REPO / "data"))
        self.assertEqual(mounts["/source"]["source"], str(REPO))
        self.assertTrue(mounts["/source"]["read_only"])

    def test_proxy_override_applies_only_when_selected(self):
        credentials = "http://fictional:fictional@mihomo:7890"
        plain = self.config()
        for name in ("backend", "frontend"):
            self.assertNotIn("HTTPS_PROXY", plain["services"][name]["environment"])
        proxied = self.config(runtime_proxy=True, EASYTIER_RUNTIME_PROXY=credentials)
        self.assertTrue(proxied["networks"]["proxy-net"]["external"])
        for name in ("backend", "frontend"):
            svc = proxied["services"][name]
            self.assertEqual(svc["environment"]["HTTPS_PROXY"], credentials)
            self.assertEqual(svc["environment"]["http_proxy"], credentials)
            self.assertEqual(set(svc["networks"]), {"default", "proxy-net"})
            self.assertIn("backend", svc["environment"]["NO_PROXY"])
        self.assertEqual(proxied["services"]["backend"]["environment"]["CARGO_NET_GIT_FETCH_WITH_CLI"], "true")
        self.assertEqual(proxied["services"]["frontend"]["environment"]["API_BASE_URL"], "http://backend:11211")

    def test_ws_uses_tcp_and_custom_ports(self):
        config = self.config(ET_CONFIG_SERVER_PROTOCOL="ws", CONFIG_TRANSPORT="tcp",
                             ET_CONFIG_SERVER_PORT="23020", ET_API_SERVER_PORT="12311", WEB_PORT="12310")
        backend = config["services"]["backend"]
        self.assertEqual({item["protocol"] for item in backend["ports"]}, {"tcp"})
        self.assertEqual({item["target"] for item in backend["ports"]}, {23020, 12311})
        self.assertEqual(config["services"]["frontend"]["environment"]["API_BASE_URL"], "http://backend:12311")


if __name__ == "__main__":
    unittest.main()
