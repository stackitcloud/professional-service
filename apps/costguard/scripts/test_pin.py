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

"""Tests for pin.py. Run: python3 -m unittest discover -s <this dir>"""

import contextlib
import io
import os
import sys
import tempfile
import unittest

import pin

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(
    0, os.path.join(HERE, "..", "terraform", "modules", "cloud-init", "files")
)
import costguard_install  # noqa: E402

A = "a" * 64
B = "b" * 64


def sums(version="v0.1.0", amd64=A, arm64=B):
    return (
        f"{amd64}  costguard_{version}_linux_amd64\n"
        f"{arm64}  costguard_{version}_linux_arm64\n"
    )


class VersionTest(unittest.TestCase):
    def test_valid(self):
        for v in ["v0.1.0", "v1.20.3", "v0.2.0-rc.1", "v1.0.0-beta"]:
            pin.check_version(v)

    def test_invalid(self):
        for v in [
            "0.1.0",
            "v01.0.0",
            "v1.2",
            "v1.2.3+build",
            "latest",
            "",
            "v1.2.3-",
            "pr-75",
        ]:
            with self.assertRaises(pin.PinError, msg=v):
                pin.check_version(v)


class SumsTest(unittest.TestCase):
    def test_parses_both_architectures(self):
        self.assertEqual(pin.parse_sums(sums(), "v0.1.0"), {"amd64": A, "arm64": B})

    def test_binary_mode_marker(self):
        # shasum -b writes "<hash> *<name>"
        text = f"{A} *costguard_v0.1.0_linux_amd64\n{B} *costguard_v0.1.0_linux_arm64\n"
        self.assertEqual(pin.parse_sums(text, "v0.1.0"), {"amd64": A, "arm64": B})

    def test_other_version(self):
        with self.assertRaisesRegex(pin.PinError, "another VERSION"):
            pin.parse_sums(sums(version="dev"), "v0.1.0")

    def test_missing_architecture(self):
        with self.assertRaisesRegex(pin.PinError, "no binary for arm64"):
            pin.parse_sums(f"{A}  costguard_v0.1.0_linux_amd64\n", "v0.1.0")

    def test_duplicate(self):
        with self.assertRaisesRegex(pin.PinError, "twice"):
            pin.parse_sums(sums() + f"{A}  costguard_v0.1.0_linux_amd64\n", "v0.1.0")

    def test_bad_hash(self):
        with self.assertRaises(pin.PinError):
            pin.parse_sums(sums(amd64="xyz"), "v0.1.0")


class RenderTest(unittest.TestCase):
    def test_round_trip(self):
        text = pin.render("v0.1.0", {"amd64": A, "arm64": B})
        self.assertEqual(
            pin.read_pin(text), ("v0.1.0", {"amd64": A, "arm64": B}, pin.DOWNLOAD_URL)
        )

    def test_block(self):
        # terraform fmt keeps exactly this layout (checked with 1.16.4).
        text = pin.render("v0.1.0", {"amd64": A, "arm64": B})
        self.assertIn(
            "locals {\n"
            "  release = {\n"
            '    version = "v0.1.0"\n'
            "    sha256 = {\n"
            f'      amd64 = "{A}"\n'
            f'      arm64 = "{B}"\n'
            "    }\n",
            text,
        )
        self.assertTrue(text.startswith("# Copyright 2026 Schwarz Digits"))
        self.assertTrue(text.endswith("  }\n}\n"))

    def test_refuses_bad_version(self):
        with self.assertRaises(pin.PinError):
            pin.render("v1.2", {"amd64": A, "arm64": B})

    def test_reads_empty_pin(self):
        text = 'locals {\n  release = {\n    version = ""\n    sha256  = {}\n    download_url = ""\n  }\n}\n'
        self.assertEqual(pin.read_pin(text), ("", {}, ""))

    def test_committed_pin_is_generated(self):
        # 025-release.tf is written by pin.py only: a pinned file must be
        # exactly what render() makes of its own values.
        with open(pin.PIN_FILE, encoding="utf-8") as f:
            text = f.read()
        version, pinned, _ = pin.read_pin(text)
        if version:
            self.assertEqual(text, pin.render(version, pinned))


class UrlTest(unittest.TestCase):
    def test_escaped_tag(self):
        urls = pin.download_urls("v0.1.0", {"amd64": A, "arm64": B}, pin.DOWNLOAD_URL)
        self.assertEqual(
            urls[0],
            (
                "amd64",
                "https://professional-service.git.onstackit.cloud/professional-service-best-practices/"
                "professional-service/releases/download/apps%2Fcostguard%2Fv0.1.0/costguard_v0.1.0_linux_amd64",
                A,
            ),
        )

    def test_same_url_as_the_installer(self):
        # The release job checks the URL the server will download.
        for arch, url, _ in pin.download_urls(
            "v0.2.0-rc.1", {"amd64": A, "arm64": B}, pin.DOWNLOAD_URL
        ):
            self.assertEqual(
                url,
                costguard_install.download_url(pin.DOWNLOAD_URL, "v0.2.0-rc.1", arch),
            )


class CompareTest(unittest.TestCase):
    def test_equal(self):
        pin.compare("v0.1.0", {"amd64": A, "arm64": B}, {"amd64": A, "arm64": B})

    def test_differs(self):
        with self.assertRaisesRegex(pin.PinError, f"arm64: pinned {B}, built {A}"):
            pin.compare("v0.1.0", {"amd64": A, "arm64": B}, {"amd64": A, "arm64": A})


class MainTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)
        self.pin_file = os.path.join(self.dir.name, "025-release.tf")
        self.sums = os.path.join(self.dir.name, "SHA256SUMS")
        with open(self.sums, "w", encoding="utf-8") as f:
            f.write(sums())

    def run_main(self, *argv):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stderr(err):
            code = pin.main(list(argv), pin_file=self.pin_file, out=out)
        return code, out.getvalue(), err.getvalue()

    def test_write_version_urls_check(self):
        code, _, _ = self.run_main("write", "--version", "v0.1.0", "--sums", self.sums)
        self.assertEqual(code, 0)
        self.assertEqual(self.run_main("version")[1], "v0.1.0\n")
        lines = self.run_main("urls")[1].splitlines()
        self.assertEqual([line.split()[0] for line in lines], ["amd64", "arm64"])
        self.assertEqual(self.run_main("check", "--sums", self.sums)[0], 0)

    def test_check_mismatch(self):
        self.run_main("write", "--version", "v0.1.0", "--sums", self.sums)
        with open(self.sums, "w", encoding="utf-8") as f:
            f.write(sums(arm64=A))
        code, _, err = self.run_main("check", "--sums", self.sums)
        self.assertEqual(code, 1)
        self.assertIn("don't match the pin", err)

    def test_write_refuses_bad_version(self):
        code, _, err = self.run_main("write", "--version", "0.1.0", "--sums", self.sums)
        self.assertEqual(code, 1)
        self.assertIn("not vMAJOR.MINOR.PATCH", err)
        self.assertFalse(os.path.exists(self.pin_file))

    def test_nothing_pinned(self):
        with open(self.pin_file, "w", encoding="utf-8") as f:
            f.write(
                'locals {\n  release = {\n    version = ""\n    sha256  = {}\n    download_url = ""\n  }\n}\n'
            )
        self.assertEqual(self.run_main("version"), (0, "\n", ""))
        code, _, err = self.run_main("urls")
        self.assertEqual(code, 1)
        self.assertIn("pins no release", err)


if __name__ == "__main__":
    unittest.main()
