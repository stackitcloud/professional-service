#!/usr/bin/env python3
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

"""Ensure every example ships a MAINTAINERS.md and Terraform lock files.

For each example (directory directly below examples/) that one of the given
files belongs to:
  MAINTAINERS.md       — must exist and list at least one maintainer with an
                         email address (e.g. "- Jane Doe (jane.doe@example.com)")
  .terraform.lock.hcl  — must exist in every Terraform root directory, i.e.
                         every directory with .tf files that is not used as a
                         local module (source = "./modules/x") by another one.

Exit 0 when all examples pass, 1 when at least one file is missing.

Usage (pre-commit passes matching filenames as positional arguments):
    python3 .github/scripts/check_example_files.py <file> [<file>…]
"""

import re
import sys
from pathlib import Path

# A list entry ("- " or "* ") that contains an email address.
_MAINTAINER_RE = re.compile(
    r"^\s*[-*]\s+.*\b[\w.%+-]+@[\w-]+(\.[\w-]+)+\b", re.MULTILINE
)
# source = "./..." or "../..." inside a module block.
_LOCAL_SOURCE_RE = re.compile(r'^\s*source\s*=\s*"(\.{1,2}/[^"]*)"', re.MULTILINE)


def _examples(files: list[str]) -> list[Path]:
    """Return the example directories the given files belong to."""
    examples = set()
    for raw in files:
        parts = Path(raw).parts
        if len(parts) > 2 and parts[0] == "examples":
            examples.add(Path(*parts[:2]))
    return sorted(e for e in examples if e.is_dir())


def _terraform_roots(example: Path) -> list[Path]:
    """Directories with .tf files that are not used as a local module."""
    dirs = {
        tf.parent
        for tf in example.rglob("*.tf")
        # skip the local .terraform/ provider cache and other hidden directories
        if not any(p.startswith(".") for p in tf.relative_to(example).parts[:-1])
    }
    modules = {
        (d / src).resolve()
        for d in dirs
        for tf in d.glob("*.tf")
        for src in _LOCAL_SOURCE_RE.findall(tf.read_text(errors="ignore"))
    }
    return sorted(d for d in dirs if d.resolve() not in modules)


def check(example: Path) -> int:
    """Check one example and return the number of violations."""
    violations = 0
    maintainers = example / "MAINTAINERS.md"
    if not maintainers.is_file():
        print(f"ERROR: {maintainers} does not exist")
        violations += 1
    elif not _MAINTAINER_RE.search(maintainers.read_text(errors="ignore")):
        print(
            f"ERROR: {maintainers} lists no maintainer, "
            "add at least one entry like `- Jane Doe (jane.doe@example.com)`"
        )
        violations += 1
    for root in _terraform_roots(example):
        if not (root / ".terraform.lock.hcl").is_file():
            print(
                f"ERROR: {root}/.terraform.lock.hcl is missing, "
                f"run `terraform init` in {root} and commit the lock file"
            )
            violations += 1
    return violations


def main() -> None:
    violations = sum(check(e) for e in _examples(sys.argv[1:]))
    sys.exit(0 if violations == 0 else 1)


if __name__ == "__main__":
    main()
