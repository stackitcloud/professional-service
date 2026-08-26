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

"""Check the codebase for open TODO comments.

Searches all text files recursively for `# TODO` and `// TODO` patterns.
Skips binary files, .git/, and .github/ directories.

Exit 0 if no TODOs are found, 1 if any are found (with a clear report).

Run:
    python3 .github/scripts/check_todos.py
"""

import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent

# Directories to skip entirely
_SKIP_DIRS = frozenset({".git", ".github"})

# Pattern matching `# TODO` and `// TODO` (with optional space)
_TODO_PATTERN = re.compile(r"(#|//) ?TODO", re.IGNORECASE)


def _is_binary(path: Path) -> bool:
    """Return True if the file appears to be binary."""
    try:
        with path.open("rb") as f:
            return b"\x00" in f.read(8192)
    except OSError:
        return True


def main() -> None:
    findings: list[str] = []

    for path in sorted(REPO_ROOT.rglob("*")):
        if not path.is_file():
            continue

        # Skip excluded directories
        if any(part in _SKIP_DIRS for part in path.parts):
            continue

        if _is_binary(path):
            continue

        try:
            lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
        except OSError:
            continue

        for lineno, line in enumerate(lines, start=1):
            if _TODO_PATTERN.search(line):
                rel = path.relative_to(REPO_ROOT)
                findings.append(f"  {rel}:{lineno}: {line.strip()}")

    if findings:
        print(
            f"Error: {len(findings)} TODO(s) found. Please resolve them before merging.\n"
        )
        print("\n".join(findings))
        sys.exit(1)

    print("No TODOs found. ✓")


if __name__ == "__main__":
    main()
