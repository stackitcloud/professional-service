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

"""Writes and reads the release pin, terraform/025-release.tf: the version
the Terraform code installs, the SHA-256 of each binary and where the release
pipeline publishes them. Used by `make pin`, `make pin-check` and the release
workflow.

  pin.py write --version vX.Y.Z --sums dist/SHA256SUMS   writes the pin
  pin.py version                                          prints the pinned version ("" if none)
  pin.py check --sums dist/SHA256SUMS                     compares a build with the pin
  pin.py urls                                             prints "<arch> <url> <sha256>" per binary

Standard library only.
"""

import argparse
import os
import re
import sys
import urllib.parse

HERE = os.path.dirname(os.path.abspath(__file__))
PIN_FILE = os.path.join(HERE, "..", "terraform", "025-release.tf")
BINARY = "costguard"
ARCHITECTURES = ("amd64", "arm64")
# The Forgejo release of tag apps/costguard/<version>. The tag's slashes stay
# escaped: the download route takes the tag as one path segment.
DOWNLOAD_URL = (
    "https://professional-service.git.onstackit.cloud/professional-service-best-practices/"
    "professional-service/releases/download/apps%2Fcostguard%2F{version}/"
)
# Semantic version with a v, optionally a pre-release (v0.2.0-rc.1); no build
# metadata, because "+" would need escaping in the tag and the URL.
VERSION_RE = re.compile(
    r"^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$"
)
HEX64_RE = re.compile(r"^[0-9a-f]{64}$")

HEADER = """\
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


# The costguard release this Terraform code installs. Written by
# `make pin VERSION=vX.Y.Z` (scripts/pin.py), never by hand. When a commit
# that changes this file is pushed, the release workflow rebuilds the
# binaries, refuses to release if their hashes differ (the build is
# reproducible) and publishes them under the tag apps/costguard/<version>.
# So a checkout of a release tag installs exactly the binary built from that
# tag, and a changed download can't run.
#
# main keeps the latest release's pin until the next release: install from a
# release tag, not from main. A test build sets binary_override instead.
"""


class PinError(Exception):
    """A problem with the version, the sums or the pin file."""


def check_version(version):
    if not VERSION_RE.match(version):
        raise PinError(
            f"version {version!r} is not vMAJOR.MINOR.PATCH (optionally -pre.release), e.g. v0.1.0"
        )


def file_name(version, arch):
    return f"{BINARY}_{version}_linux_{arch}"


def parse_sums(text, version):
    """The hashes per architecture from a SHA256SUMS written by make dist
    for this version; every architecture exactly once, nothing else."""
    sums = {}
    for line in text.splitlines():
        if not line.strip():
            continue
        parts = line.split()
        if len(parts) != 2:
            raise PinError(f"SHA256SUMS: unreadable line {line!r}")
        digest, name = parts[0].lower(), parts[1].lstrip("*")
        arch = next(
            (a for a in ARCHITECTURES if name == file_name(version, a)),
            None,
        )
        if arch is None or not HEX64_RE.match(digest):
            raise PinError(
                f"SHA256SUMS: {name!r} is not a binary of {version} (built with another VERSION?)"
            )
        if arch in sums:
            raise PinError(f"SHA256SUMS: {name} is listed twice")
        sums[arch] = digest
    missing = [a for a in ARCHITECTURES if a not in sums]
    if missing:
        raise PinError(f"SHA256SUMS: no binary for {', '.join(missing)}")
    return sums


def render(version, sums):
    check_version(version)
    lines = [
        HEADER,
        "locals {",
        "  release = {",
        f'    version = "{version}"',
        "    sha256 = {",
    ]
    width = max(len(a) for a in ARCHITECTURES)
    lines += [f'      {a.ljust(width)} = "{sums[a]}"' for a in ARCHITECTURES]
    lines += [
        "    }",
        "    # Where the release workflow publishes the binaries; {version} is",
        "    # filled in, the file name costguard_<version>_linux_<arch> appended.",
        f'    download_url = "{DOWNLOAD_URL}"',
        "  }",
        "}",
    ]
    return "\n".join(lines) + "\n"


def read_pin(text):
    """(version, {arch: sha256}, download_url) from the pin file; version ""
    when nothing is pinned."""
    version = re.search(r'^\s*version\s*=\s*"([^"]*)"', text, re.M)
    if not version:
        raise PinError("025-release.tf: no version line")
    block = re.search(r"^\s*sha256\s*=\s*\{(.*?)\}", text, re.M | re.S)
    if not block:
        raise PinError("025-release.tf: no sha256 map")
    sums = dict(re.findall(r'(\w+)\s*=\s*"([0-9a-f]*)"', block.group(1)))
    url = re.search(r'^\s*download_url\s*=\s*"([^"]*)"', text, re.M)
    if not url:
        raise PinError("025-release.tf: no download_url line")
    return version.group(1), sums, url.group(1)


def download_urls(version, sums, template):
    """[(arch, url, sha256)] as the server's installer builds the URLs."""
    base = template.replace("{version}", urllib.parse.quote(version, safe=""))
    return [
        (a, base.rstrip("/") + "/" + file_name(version, a), sums[a])
        for a in ARCHITECTURES
        if a in sums
    ]


def compare(pinned_version, pinned, built):
    """Raises PinError unless the build has the pinned hashes."""
    diffs = [
        f"{file_name(pinned_version, a)}: pinned {pinned.get(a, '-')}, built {built.get(a, '-')}"
        for a in ARCHITECTURES
        if pinned.get(a) != built.get(a)
    ]
    if diffs:
        raise PinError(
            "the binaries built from this commit don't match the pin in 025-release.tf, so "
            f"{pinned_version} can't be released from it. The Go code changed after `make pin`, or "
            "the pin was built with another Go toolchain. Run make pin again and commit the result.\n  "
            + "\n  ".join(diffs)
        )


def read_file(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def main(argv=None, pin_file=PIN_FILE, out=sys.stdout):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = parser.add_subparsers(dest="command", required=True)
    write = sub.add_parser("write")
    write.add_argument("--version", required=True)
    write.add_argument("--sums", required=True)
    sub.add_parser("version")
    check = sub.add_parser("check")
    check.add_argument("--sums", required=True)
    sub.add_parser("urls")
    args = parser.parse_args(argv)

    try:
        if args.command == "write":
            check_version(args.version)
            sums = parse_sums(read_file(args.sums), args.version)
            content = render(args.version, sums)
            tmp = pin_file + ".new"
            with open(tmp, "w", encoding="utf-8") as f:
                f.write(content)
            os.replace(tmp, pin_file)
            print(f"pinned {args.version} in {os.path.relpath(pin_file)}", file=out)
            return 0

        version, pinned, url = read_pin(read_file(pin_file))
        if args.command == "version":
            print(version, file=out)
            return 0
        if not version:
            raise PinError("025-release.tf pins no release")
        if args.command == "urls":
            for arch, u, digest in download_urls(version, pinned, url):
                print(arch, u, digest, file=out)
            return 0
        # check
        compare(version, pinned, parse_sums(read_file(args.sums), version))
        print(f"the build matches the pin of {version}", file=out)
        return 0
    except (PinError, OSError) as err:
        print(f"pin: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
