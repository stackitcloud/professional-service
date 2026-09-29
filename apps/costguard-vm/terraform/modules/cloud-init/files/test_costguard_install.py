# Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Tests for costguard_install.py. Run: python3 -m unittest discover -s <this dir>"""

import contextlib
import hashlib
import io
import json
import os
import stat
import tempfile
import unittest
import urllib.error

import costguard_install as ci

BINARY = b"\x7fELF fake costguard-vm"
SHA = hashlib.sha256(BINARY).hexdigest()


def config(**over):
    cfg = {
        "version": "v0.1.0",
        "url": "https://dl.example/{version}/",
        "sha256": {"amd64": SHA},
        "output": "slack",
    }
    cfg.update(over)
    return cfg


class InstallTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)
        self.sleeps = []

    def run_install(self, cfg, get, machine="x86_64"):
        ci.install(
            cfg,
            target_dir=self.dir.name,
            machine=machine,
            sleep=self.sleeps.append,
            get=get,
        )

    def test_installs_verified_binary(self):
        urls = []
        self.run_install(config(), lambda url: urls.append(url) or BINARY)
        self.assertEqual(
            urls, ["https://dl.example/v0.1.0/costguard-vm_v0.1.0_linux_amd64"]
        )
        target = os.path.join(self.dir.name, "costguard-vm")
        with open(target, "rb") as f:
            self.assertEqual(f.read(), BINARY)
        self.assertEqual(stat.S_IMODE(os.stat(target).st_mode), 0o755)
        self.assertFalse(os.path.exists(target + ".new"))
        self.assertEqual(self.sleeps, [])

    def test_version_in_url_is_escaped(self):
        urls = []
        cfg = config(
            version="apps/costguard-vm/v0.1.0", url="https://dl.example/{version}"
        )
        with self.assertRaises(
            ci.InstallError
        ):  # the fake hash doesn't match; only the URL matters here
            self.run_install(cfg, lambda url: urls.append(url) or b"x")
        self.assertTrue(
            urls[0].startswith("https://dl.example/apps%2Fcostguard-vm%2Fv0.1.0/")
        )

    def test_hash_mismatch_fails_at_once(self):
        calls = []
        with self.assertRaises(ci.InstallError) as ctx:
            self.run_install(config(), lambda url: calls.append(url) or b"tampered")
        self.assertIn("does not match the pinned SHA-256 hash", str(ctx.exception))
        self.assertEqual(len(calls), 1)
        self.assertEqual(os.listdir(self.dir.name), [])

    def test_retries_then_gives_up(self):
        def get(url):
            raise urllib.error.HTTPError(url, 404, "Not Found", {}, io.BytesIO())

        with self.assertRaises(ci.InstallError) as ctx:
            self.run_install(config(), get)
        self.assertEqual(self.sleeps, [ci.PAUSE_SECONDS] * 3)
        self.assertIn("failed 4 times, last error: HTTP 404", str(ctx.exception))
        self.assertNotIn("dl.example", str(ctx.exception))

    def test_recovers_after_transient_errors(self):
        answers = [
            urllib.error.URLError("temporary failure in name resolution"),
            TimeoutError(),
            BINARY,
        ]

        def get(url):
            a = answers.pop(0)
            if isinstance(a, Exception):
                raise a
            return a

        self.run_install(config(), get)
        self.assertEqual(self.sleeps, [ci.PAUSE_SECONDS] * 2)

    def test_arm64_uses_its_own_hash(self):
        urls = []
        self.run_install(
            config(sha256={"arm64": SHA}),
            lambda url: urls.append(url) or BINARY,
            machine="aarch64",
        )
        self.assertTrue(urls[0].endswith("_linux_arm64"))

    def test_architecture_without_pin(self):
        with self.assertRaises(ci.InstallError) as ctx:
            self.run_install(config(), lambda url: BINARY, machine="aarch64")
        self.assertIn("(arm64)", str(ctx.exception))

    def test_refuses_plain_http(self):
        with self.assertRaises(ci.InstallError):
            self.run_install(
                config(url="http://dl.example/{version}"), lambda url: BINARY
            )
        # http on the local machine is allowed (tests).
        self.run_install(
            config(url="http://127.0.0.1:8080/{version}"), lambda url: BINARY
        )


class NotifyTest(unittest.TestCase):
    def test_payload_shapes(self):
        self.assertEqual(ci.payload("slack", "hi"), {"text": "hi"})
        self.assertEqual(ci.payload("googlechat", "hi"), {"text": "hi"})
        card = ci.payload("teams", "hi")["attachments"][0]["content"]
        self.assertEqual(card["type"], "AdaptiveCard")
        self.assertEqual(card["body"][0]["text"], "hi")

    def test_posts_json(self):
        seen = []

        class Resp:
            def __enter__(self):
                return self

            def __exit__(self, *a):
                return False

        def opener(req, timeout):
            seen.append(
                (req.full_url, req.get_header("Content-type"), json.loads(req.data))
            )
            return Resp()

        self.assertTrue(
            ci.notify("slack", "https://hooks.example/x", "boom", opener=opener)
        )
        self.assertEqual(
            seen, [("https://hooks.example/x", "application/json", {"text": "boom"})]
        )

    def test_retries_and_never_logs_the_webhook(self):
        sleeps = []

        def opener(req, timeout):
            raise urllib.error.URLError("connection refused")

        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertFalse(
                ci.notify(
                    "slack",
                    "https://hooks.example/secret-token",
                    "boom",
                    sleep=sleeps.append,
                    opener=opener,
                )
            )
        self.assertEqual(sleeps, [ci.POST_PAUSE_SECONDS] * 2)
        self.assertNotIn("secret-token", out.getvalue())

    def test_refuses_insecure_webhook(self):
        self.assertFalse(
            ci.notify("slack", "http://hooks.example/x", "boom", opener=None)
        )
        self.assertFalse(ci.notify("slack", "", "boom", opener=None))


class FailureTextTest(unittest.TestCase):
    def test_names_the_fix(self):
        text = ci.failure_text("v0.1.0", "HTTP 404")
        self.assertIn("costguard-vm v0.1.0 could not be installed", text)
        self.assertIn("terraform apply -replace=stackit_server.costguard", text)


if __name__ == "__main__":
    unittest.main()
