"""Offline tests for the checksum-gated two-source protoc installer."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

INSTALLER = Path(__file__).resolve().parents[1] / "install-protoc.sh"


class ProtocInstallerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.target = self.root / "install"
        self.target.mkdir()
        self.log = self.root / "curl.log"
        self.env = {
            **os.environ,
            "PATH": f"{self.bin}:{os.environ['PATH']}",
            "TEST_CURL_LOG": str(self.log),
        }

    def fake(self, name, content):
        path = self.bin / name
        path.write_text("#!/bin/sh\nset -eu\n" + content + "\n")
        path.chmod(0o755)

    def call(self, version="35.1"):
        return subprocess.run(
            ["sh", str(INSTALLER), version, str(self.target)],
            capture_output=True,
            text=True,
            env=self.env,
            timeout=5,
        )

    def test_unsupported_version_requires_pinned_digest(self):
        result = self.call("35.2")
        self.assertEqual(result.returncode, 2)
        self.assertIn("Add its official release SHA-256", result.stderr)

    def test_both_sources_unavailable_fails_closed(self):
        self.fake(
            "curl",
            'for arg do case "$arg" in https://*) echo "$arg" >> "$TEST_CURL_LOG" ;; esac; done; exit 35',
        )
        result = self.call()
        self.assertEqual(result.returncode, 1)
        self.assertIn("both unavailable", result.stderr)
        urls = self.log.read_text().splitlines()
        self.assertEqual(len(urls), 2)
        self.assertIn("github.com", urls[0])
        self.assertIn("sourceforge.net", urls[1])

    def test_corrupt_mirror_data_never_installs(self):
        self.fake(
            "curl",
            """
out=; url=; prev=
for arg do
    case "$prev" in --output) out=$arg ;; esac
    case "$arg" in https://*) url=$arg ;; esac
    prev=$arg
done
printf '%s\n' "$url" >> "$TEST_CURL_LOG"
case "$url" in *sourceforge*) echo 'corrupt' > "$out"; exit 0 ;; *) exit 35 ;; esac
""",
        )
        self.fake("unzip", f'echo called > "{self.root}/unzip_called"')
        result = self.call()
        self.assertEqual(result.returncode, 1)
        self.assertIn("does not match", result.stderr)
        self.assertFalse((self.root / "unzip_called").exists())

    def test_verified_mirror_fallback_installs(self):
        self.fake(
            "curl",
            """
out=; url=; prev=
for arg do
    case "$prev" in --output) out=$arg ;; esac
    case "$arg" in https://*) url=$arg ;; esac
    prev=$arg
done
printf '%s\n' "$url" >> "$TEST_CURL_LOG"
case "$url" in *sourceforge*) echo 'fixture zip' > "$out"; exit 0 ;; *) exit 35 ;; esac
""",
        )
        # Fake digest checker exercises control flow; the separate mismatch
        # case above uses the real sha256sum to verify fail-closed behavior.
        self.fake(
            "sha256sum",
            """
input=$(cat)
case "$input" in
    6930ebf62bd4ea607b98fff052596c6ee564b9835b4ce172c75a3f53ae9d91b7*) exit 0 ;;
    *) exit 1 ;;
esac
""",
        )
        self.fake(
            "unzip",
            """
dest=; prev=
for arg do
    if [ "$prev" = -d ]; then dest=$arg; fi
    prev=$arg
done
mkdir -p "$dest/bin"
printf '#!/bin/sh\necho "libprotoc 35.1"\n' > "$dest/bin/protoc"
""",
        )
        result = self.call()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("SourceForge mirror", result.stdout)
        self.assertEqual(len(self.log.read_text().splitlines()), 2)
        self.assertEqual(
            subprocess.check_output([str(self.target / "bin/protoc")], text=True).strip(),
            "libprotoc 35.1",
        )


if __name__ == "__main__":
    unittest.main()
