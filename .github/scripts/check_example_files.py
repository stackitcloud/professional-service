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

"""Verify that examples ship a MAINTAINERS.md and a Terraform lock file.

Checks (per example, i.e. every directory directly below examples/):
  MAINTAINERS.md       — must exist in the example directory
  .terraform.lock.hcl  — at least one must exist in the example if it
                         contains .tf files, so the provider versions are pinned

CI checks only the examples touched by a change, so existing examples do not
block unrelated pull requests:
    python3 .github/scripts/check_example_files.py --changed-from <git-ref>

Check every example:
    python3 .github/scripts/check_example_files.py --all

Exit 0 if all checks pass, 1 if any file is missing (with a clear report).
"""

import argparse
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
EXAMPLES_DIR = REPO_ROOT / "examples"


def _all_examples() -> list[str]:
    return sorted(
        d.name
        for d in EXAMPLES_DIR.iterdir()
        if d.is_dir() and not d.name.startswith(".")
    )


def _changed_examples(base: str) -> list[str]:
    """Return the existing examples with changes between base and HEAD."""
    result = subprocess.run(
        ["git", "diff", "--name-only", f"{base}...HEAD", "--", "examples/"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    names = {
        Path(line).parts[1]
        for line in result.stdout.splitlines()
        if len(Path(line).parts) > 2
    }
    return sorted(n for n in names if (EXAMPLES_DIR / n).is_dir())


def _files(example: Path, pattern: str) -> list[Path]:
    """Files matching pattern, ignoring the local .terraform/ provider cache."""
    return [p for p in example.rglob(pattern) if ".terraform" not in p.parts]


def check_example(name: str) -> list[str]:
    example = EXAMPLES_DIR / name
    errors: list[str] = []
    if not (example / "MAINTAINERS.md").is_file():
        errors.append(f"  examples/{name}/MAINTAINERS.md — file does not exist")
    if _files(example, "*.tf") and not _files(example, ".terraform.lock.hcl"):
        errors.append(
            f"  examples/{name}/.terraform.lock.hcl — missing, run `terraform init` and commit the lock file"
        )
    return errors


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--all", action="store_true", help="check every example")
    group.add_argument(
        "--changed-from",
        metavar="REF",
        help="check only examples changed between REF and HEAD",
    )
    args = parser.parse_args()

    examples = _all_examples() if args.all else _changed_examples(args.changed_from)
    if not examples:
        print("No examples to check.")
        sys.exit(0)

    all_errors: list[str] = []
    for name in examples:
        all_errors.extend(check_example(name))

    if all_errors:
        print("Examples missing required files:")
        print("\n".join(all_errors))
        print()
        print(f"Found {len(all_errors)} issue(s).")
        sys.exit(1)

    print(
        f"All {len(examples)} checked example(s) have MAINTAINERS.md and a lock file. ✓"
    )


if __name__ == "__main__":
    main()
