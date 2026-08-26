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

"""Verify that every example, module, and script has tags defined.

Checks:
  examples/*/README.md  — line 1 must contain <!-- tags: ... -->
  modules/*/README.md   — line 1 must contain <!-- tags: ... -->
  scripts/README.md     — every .sh row in the overview table must have
                          at least one tag in the Tags column

Exit 0 if all checks pass, 1 if any tags are missing (with a clear report).

Run:
    python3 .github/scripts/check_readme_tags.py
"""

import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
EXAMPLES_DIR = REPO_ROOT / "examples"
MODULES_DIR = REPO_ROOT / "modules"
SCRIPTS_DIR = REPO_ROOT / "scripts"

_TAG_COMMENT = re.compile(r"<!--\s*tags:\s*(.+?)\s*-->")

# Tags must be lowercase and may only contain letters, digits, and hyphens.
# Multi-word tags use hyphens (e.g. "object-storage"), never underscores or camelCase.
_VALID_TAG = re.compile(r"^[a-z0-9][a-z0-9-]*$")


def _parse_tag_comment(readme_path: Path) -> tuple[bool, list[str]]:
    """Return (has_comment, tags) from the first line of the file.

    has_comment is False if the <!-- tags: --> marker is absent or empty.
    tags is the list of individual tag strings found (may be malformed).
    """
    try:
        first_line = readme_path.read_text(encoding="utf-8").split("\n", 1)[0].strip()
    except OSError:
        return False, []
    m = _TAG_COMMENT.match(first_line)
    if not m or not m.group(1).strip():
        return False, []
    tags = [t.strip() for t in m.group(1).split(",") if t.strip()]
    return True, tags


def _has_tag_comment(readme_path: Path) -> bool:
    """Return True if the first line of the file contains a non-empty <!-- tags: --> comment."""
    has_comment, _ = _parse_tag_comment(readme_path)
    return has_comment


def _malformed_tags(tags: list[str]) -> list[str]:
    """Return tags that violate the naming convention (lowercase, hyphen-separated)."""
    return [t for t in tags if not _VALID_TAG.match(t)]


def check_examples() -> list[str]:
    errors: list[str] = []
    for d in sorted(EXAMPLES_DIR.iterdir()):
        if not d.is_dir() or d.name.startswith("."):
            continue
        readme = d / "README.md"
        if not readme.exists():
            # Examples without a README are skipped by the generator too — not an error here.
            continue
        has_comment, tags = _parse_tag_comment(readme)
        if not has_comment:
            errors.append(
                f"  examples/{d.name}/README.md — missing <!-- tags: ... --> on line 1"
            )
            continue
        bad = _malformed_tags(tags)
        if bad:
            errors.append(
                f"  examples/{d.name}/README.md — tag(s) must be lowercase and hyphen-separated: {bad}"
            )
    return errors


def check_modules() -> list[str]:
    errors: list[str] = []
    for d in sorted(MODULES_DIR.iterdir()):
        if not d.is_dir() or d.name.startswith("."):
            continue
        readme = d / "README.md"
        if not readme.exists():
            errors.append(f"  modules/{d.name}/README.md — file does not exist")
            continue
        has_comment, tags = _parse_tag_comment(readme)
        if not has_comment:
            errors.append(
                f"  modules/{d.name}/README.md — missing <!-- tags: ... --> on line 1"
            )
            continue
        bad = _malformed_tags(tags)
        if bad:
            errors.append(
                f"  modules/{d.name}/README.md — tag(s) must be lowercase and hyphen-separated: {bad}"
            )
    return errors


def check_scripts() -> list[str]:
    """Verify every .sh file is listed in the overview table with at least one tag."""
    errors: list[str] = []
    scripts_readme = SCRIPTS_DIR / "README.md"
    if not scripts_readme.exists():
        return [f"  scripts/README.md — file does not exist"]

    content = scripts_readme.read_text(encoding="utf-8")

    # Parse the overview table: collect {filename: tags_list}
    table: dict[str, list[str]] = {}
    for line in content.splitlines():
        stripped = line.strip()
        if not stripped.startswith("|"):
            continue
        cells = [c.strip() for c in stripped.split("|")]
        if len(cells) < 4:
            continue
        m = re.search(r"`([^`]+\.sh)`", cells[1])
        if not m:
            continue
        filename = m.group(1)
        # Tags column is cells[4] (after: '', script, purpose, tools, tags, '')
        tags_raw = re.sub(r"`", "", cells[4]).strip() if len(cells) > 4 else ""
        tags = [
            t.strip() for t in tags_raw.split(",") if t.strip() and t.strip() != "Tags"
        ]
        table[filename] = tags

    for script in sorted(SCRIPTS_DIR.glob("*.sh")):
        if script.name not in table:
            errors.append(
                f"  scripts/{script.name} — not listed in scripts/README.md overview table"
            )
        elif not table[script.name]:
            errors.append(
                f"  scripts/{script.name} — Tags column is empty in scripts/README.md"
            )
        else:
            bad = _malformed_tags(table[script.name])
            if bad:
                errors.append(
                    f"  scripts/{script.name} — tag(s) must be lowercase and hyphen-separated: {bad}"
                )
    return errors


def main() -> None:
    all_errors: list[str] = []

    example_errors = check_examples()
    module_errors = check_modules()
    script_errors = check_scripts()

    if example_errors:
        print("Examples missing tags:")
        print("\n".join(example_errors))
        all_errors.extend(example_errors)

    if module_errors:
        print("Modules missing tags:")
        print("\n".join(module_errors))
        all_errors.extend(module_errors)

    if script_errors:
        print("Scripts missing tags:")
        print("\n".join(script_errors))
        all_errors.extend(script_errors)

    if all_errors:
        print()
        print(f"Found {len(all_errors)} issue(s).")
        print()
        print("Tag format rules:")
        print("  - Must be lowercase letters, digits, and hyphens only")
        print(
            "  - Multi-word tags use hyphens:  object-storage  NOT  objectStorage or object_storage"
        )
        print()
        print(
            "Examples and modules: add a tag comment as the very first line of README.md:"
        )
        print("  <!-- tags: ske, velero, backup, object-storage, kubernetes -->")
        print()
        print("Recommended tags: ske, iaas, dbaas, iam, alb, waf, vpn, cdn, sfs,")
        print(
            "  object-storage, block-storage, secrets-manager, load-balancer, kubernetes,"
        )
        print("  otel, observability, encryption, tls, cert, pki, workload-identity,")
        print("  velero, backup, vault, nginx, ha, vrrp, layer4, layer7, gpu, edge,")
        print(
            "  terraform, backend, s3, nfs, aws, azure, arc, migration, windows, byol"
        )
        print()
        print(
            "Scripts: add or complete the Tags column in the scripts/README.md overview table."
        )
        sys.exit(1)

    counts = (
        sum(
            1
            for d in EXAMPLES_DIR.iterdir()
            if d.is_dir() and not d.name.startswith(".") and (d / "README.md").exists()
        ),
        sum(
            1
            for d in MODULES_DIR.iterdir()
            if d.is_dir() and not d.name.startswith(".")
        ),
        len(list(SCRIPTS_DIR.glob("*.sh"))),
    )
    print(
        f"All tags present: {counts[0]} examples, {counts[1]} modules, {counts[2]} scripts. ✓"
    )


if __name__ == "__main__":
    main()
