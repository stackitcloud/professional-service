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

"""Verify that examples ship a MAINTAINERS.md and Terraform lock files.

Checks (per example, i.e. every directory directly below examples/):
  MAINTAINERS.md       — must exist in the example directory and list at least
                         one maintainer with an email address
                         (e.g. "- Jane Doe (jane.doe@example.com)")
  .terraform.lock.hcl  — must exist in every Terraform root directory of the
                         example, so the provider versions are pinned. A root is
                         any directory with .tf files that is not used as a
                         local module (module "x" { source = "./modules/x" })
                         by another directory of the example.

CI checks only the examples touched by a change, so existing examples do not
block unrelated pull requests:
    python3 .github/scripts/check_example_files.py --changed-from <git-ref>

If <git-ref> is empty, all zeros (first push of a branch) or not in the
history (force push), only the examples changed by the HEAD commit are checked.

Check every example:
    python3 .github/scripts/check_example_files.py --all

Exit 0 if all checks pass, 1 if anything is missing (with a clear report).
"""

import argparse
import re
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
EXAMPLES_DIR = REPO_ROOT / "examples"

# A list entry ("- " or "* ") that contains an email address.
MAINTAINER_RE = re.compile(
    r"^\s*[-*]\s+.*\b[\w.%+-]+@[\w-]+(\.[\w-]+)+\b", re.MULTILINE
)
# source = "./..." or "../..." inside a module block.
LOCAL_SOURCE_RE = re.compile(r'^\s*source\s*=\s*"(\.{1,2}/[^"]*)"', re.MULTILINE)


def _git(*args: str) -> subprocess.CompletedProcess:
    return subprocess.run(["git", *args], cwd=REPO_ROOT, capture_output=True, text=True)


def _all_examples() -> list[str]:
    return sorted(
        d.name
        for d in EXAMPLES_DIR.iterdir()
        if d.is_dir() and not d.name.startswith(".")
    )


def _diff_range(base: str) -> str:
    """Return the git range to diff, falling back to the HEAD commit."""
    if base and set(base) != {"0"}:
        if _git("cat-file", "-e", f"{base}^{{commit}}").returncode == 0:
            return f"{base}...HEAD"
        print(
            f"Warning: base '{base}' is not in the history, checking the HEAD commit only."
        )
    else:
        print("Warning: no base commit given, checking the HEAD commit only.")
    if _git("rev-parse", "--verify", "--quiet", "HEAD~1").returncode == 0:
        return "HEAD~1...HEAD"
    # Root commit: compare against the empty tree.
    return f"{_git('hash-object', '-t', 'tree', '/dev/null').stdout.strip()}...HEAD"


def _changed_examples(base: str) -> list[str]:
    """Return the existing examples with changes between base and HEAD."""
    diff_range = _diff_range(base)
    result = _git("diff", "--name-only", diff_range, "--", "examples/")
    if result.returncode != 0:
        print(f"Error: git diff {diff_range} failed:\n{result.stderr.strip()}")
        sys.exit(2)
    names = {
        Path(line).parts[1]
        for line in result.stdout.splitlines()
        if len(Path(line).parts) > 2
    }
    return sorted(n for n in names if (EXAMPLES_DIR / n).is_dir())


def _ignored(path: Path, example: Path) -> bool:
    """Skip the local .terraform/ provider cache and other hidden directories."""
    return any(part.startswith(".") for part in path.relative_to(example).parts[:-1])


def _tf_dirs(example: Path) -> set[Path]:
    return {p.parent for p in example.rglob("*.tf") if not _ignored(p, example)}


def _terraform_roots(example: Path) -> list[Path]:
    """Directories with .tf files that are not used as a local module."""
    dirs = _tf_dirs(example)
    modules: set[Path] = set()
    for d in dirs:
        for tf in d.glob("*.tf"):
            for src in LOCAL_SOURCE_RE.findall(tf.read_text(errors="ignore")):
                modules.add((d / src).resolve())
    return sorted(d for d in dirs if d.resolve() not in modules)


def check_example(name: str) -> list[str]:
    example = EXAMPLES_DIR / name
    errors: list[str] = []
    maintainers = example / "MAINTAINERS.md"
    if not maintainers.is_file():
        errors.append(f"  examples/{name}/MAINTAINERS.md — file does not exist")
    elif not MAINTAINER_RE.search(maintainers.read_text(errors="ignore")):
        errors.append(
            f"  examples/{name}/MAINTAINERS.md — no maintainer listed, "
            "add at least one entry like `- Jane Doe (jane.doe@example.com)`"
        )
    for root in _terraform_roots(example):
        if not (root / ".terraform.lock.hcl").is_file():
            rel = root.relative_to(REPO_ROOT)
            errors.append(
                f"  {rel}/.terraform.lock.hcl — missing, run `terraform init` in {rel} and commit the lock file"
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
        f"All {len(examples)} checked example(s) have a MAINTAINERS.md with maintainers "
        "and a lock file in every Terraform root. ✓"
    )


if __name__ == "__main__":
    main()
