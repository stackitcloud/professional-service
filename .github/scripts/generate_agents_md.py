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

"""Generate AGENTS.md from the current repository state.

Crawls examples/, scripts/, and modules/ and writes AGENTS.md at the repo root.
The generated file embeds a full index (name, tags, description) so AI coding
assistants can find relevant content without querying GitHub — falling back to
the API only for resources added after the last generation.

Run:
    python3 .github/scripts/generate_agents_md.py

Check (CI — exits 1 if AGENTS.md is stale):
    python3 .github/scripts/generate_agents_md.py --check
"""

import argparse
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
EXAMPLES_DIR = REPO_ROOT / "examples"
SCRIPTS_DIR = REPO_ROOT / "scripts"
MODULES_DIR = REPO_ROOT / "modules"
OUTPUT = REPO_ROOT / "AGENTS.md"

GITHUB_RAW = "https://raw.githubusercontent.com/stackitcloud/professional-service/main"
GITHUB_API = "https://api.github.com/repos/stackitcloud/professional-service/contents"

# Fallback tag vocabulary for READMEs without a <!-- tags: --> comment.
# Canonical tag vocabulary.  Rules:
#   - lowercase only
#   - multi-word tags use hyphens  (e.g. "object-storage", not "objectstorage")
#   - use STACKIT product shorthand where widely recognised (ske, iaas, dbaas, alb, waf)
#   - use the protocol/tool name for third-party integrations (velero, vault, otel, nginx)
#
# This list is the fallback used when a README has no <!-- tags: --> comment.
# It is also referenced in the contributor docs inside the generated AGENTS.md.
_KNOWN_TAGS = frozenset(
    {
        # STACKIT service shorthands
        "ske",
        "iaas",
        "dbaas",
        "iam",
        "cdn",
        "alb",
        "waf",
        "vpn",
        "sfs",
        # Multi-word STACKIT concepts — always hyphenated
        "object-storage",
        "block-storage",
        "secrets-manager",
        "load-balancer",
        "landing-zone",
        "file-storage",
        # Networking patterns
        "ha",
        "vrrp",
        "hub-and-spoke",
        "layer4",
        "layer7",
        "cross-az",
        "site-to-site",
        # Kubernetes / container ecosystem
        "kubernetes",
        "k3s",
        "gpu",
        "csi",
        "ingress",
        "ephemeral",
        "kube-state-metrics",
        # Observability
        "otel",
        "telemetry",
        "observability",
        "metrics",
        "alerting",
        "log-alerts",
        # Security / identity
        "encryption",
        "cert",
        "pki",
        "tls",
        "scim",
        "oidc",
        "workload-identity",
        "kms",
        # Tools / integrations
        "velero",
        "backup",
        "vault",
        "nginx",
        "opnsense",
        "mountpoint",
        # Storage protocols
        "s3",
        "nfs",
        # IaaS patterns
        "edge",
        "image",
        "migration",
        "windows",
        "byol",
        "nested-virtualization",
        # Cloud / multi-cloud
        "aws",
        "azure",
        "arc",
        "multi-cloud",
        # Terraform patterns
        "terraform",
        "backend",
        "provider",
        "pg-backend",
    }
)

_HR = frozenset({"---", "***", "___"})


# ---------------------------------------------------------------------------
# Parsing helpers (shared with other CI scripts)
# ---------------------------------------------------------------------------


def _read_tags(readme: str, name: str) -> list[str]:
    """Return tags from the <!-- tags: ... --> comment on line 1, or infer from name."""
    first_line = readme.split("\n", 1)[0].strip()
    m = re.match(r"<!--\s*tags:\s*(.+?)\s*-->", first_line)
    if m:
        return [t.strip() for t in m.group(1).split(",") if t.strip()]
    parts = re.split(r"[-_]", name.lower())
    return [p for p in parts if p in _KNOWN_TAGS]


def _extract_description(content: str) -> str:
    """Return the first substantive paragraph from a README."""
    lines = content.splitlines()

    def _paragraph_after(start: int, end: int | None = None) -> str:
        para: list[str] = []
        for line in lines[start + 1 : end]:
            s = line.strip()
            if s.startswith("#") or s.startswith("```") or s in _HR:
                if para:
                    break
                continue
            if not s:
                if para:
                    break
                continue
            if s.startswith("|") or s.startswith(">"):
                if para:
                    break
                continue
            para.append(s)
        return " ".join(para)

    h1 = next((i for i, l in enumerate(lines) if l.strip().startswith("# ")), None)
    if h1 is None:
        return ""
    first_h2 = next(
        (i for i, l in enumerate(lines) if i > h1 and l.strip().startswith("## ")),
        len(lines),
    )
    result = _paragraph_after(h1, first_h2)
    if result:
        return result
    for kw in ("## Overview", "## Description"):
        idx = next((i for i, l in enumerate(lines) if l.strip().startswith(kw)), None)
        if idx is not None:
            result = _paragraph_after(idx)
            if result:
                return result
    for i, line in enumerate(lines):
        if line.strip().startswith("## "):
            result = _paragraph_after(i)
            if result:
                return result
    return ""


def _parse_scripts_table(readme: str) -> dict[str, tuple[str, list[str]]]:
    """Return {filename: (description, tags)} from the overview table in scripts/README.md."""
    result: dict[str, tuple[str, list[str]]] = {}
    for line in readme.splitlines():
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
        desc = re.sub(r"`([^`]*)`", r"\1", cells[2]).strip()
        if not desc or desc == "Purpose":
            continue
        tags_raw = re.sub(r"`", "", cells[4]) if len(cells) > 4 else ""
        tags = [
            t.strip() for t in tags_raw.split(",") if t.strip() and t.strip() != "Tags"
        ]
        result[filename] = (desc, tags)
    return result


# ---------------------------------------------------------------------------
# Index builders
# ---------------------------------------------------------------------------


def _build_examples_index() -> list[str]:
    lines = ["## Examples", ""]
    found = False
    for d in sorted(EXAMPLES_DIR.iterdir()):
        if not d.is_dir() or d.name.startswith("."):
            continue
        readme_path = d / "README.md"
        if not readme_path.exists():
            continue
        readme = readme_path.read_text(encoding="utf-8")
        tags = _read_tags(readme, d.name)
        desc = _extract_description(readme)
        tag_str = ", ".join(tags) if tags else "—"
        desc_str = desc.rstrip(".") if desc else "—"
        lines.append(f"- **`{d.name}`** `[{tag_str}]`  ")
        lines.append(f"  {desc_str}")
        found = True
    if not found:
        lines.append("_No examples found._")
    lines.append("")
    return lines


def _build_scripts_index() -> list[str]:
    lines = ["## Scripts", ""]
    scripts_readme_path = SCRIPTS_DIR / "README.md"
    table: dict[str, tuple[str, list[str]]] = {}
    if scripts_readme_path.exists():
        table = _parse_scripts_table(scripts_readme_path.read_text(encoding="utf-8"))
    found = False
    for f in sorted(SCRIPTS_DIR.glob("*.sh")):
        entry = table.get(f.name)
        desc = entry[0] if entry else ""
        tags = entry[1] if entry else []
        if not tags:
            parts = re.split(r"[-_]", f.stem.lower())
            tags = [p for p in parts if p in _KNOWN_TAGS]
        tag_str = ", ".join(tags) if tags else "—"
        desc_str = desc.rstrip(".") if desc else "—"
        lines.append(f"- **`{f.name}`** `[{tag_str}]`  ")
        lines.append(f"  {desc_str}")
        found = True
    if not found:
        lines.append("_No scripts found._")
    lines.append("")
    return lines


def _build_modules_index() -> list[str]:
    lines = ["## Modules", ""]
    found = False
    for d in sorted(MODULES_DIR.iterdir()):
        if not d.is_dir() or d.name.startswith("."):
            continue
        readme_path = d / "README.md"
        readme = readme_path.read_text(encoding="utf-8") if readme_path.exists() else ""
        tags = _read_tags(readme, d.name) if readme else []
        desc = _extract_description(readme) if readme else ""
        tag_str = ", ".join(tags) if tags else "—"
        desc_str = desc.rstrip(".") if desc else "—"
        lines.append(f"- **`{d.name}`** `[{tag_str}]`  ")
        lines.append(f"  {desc_str}")
        found = True
    if not found:
        lines.append("_No modules found._")
    lines.append("")
    return lines


# ---------------------------------------------------------------------------
# Template
# ---------------------------------------------------------------------------

_HEADER = """\
<!-- Generated by .github/scripts/generate_agents_md.py — do not edit manually.
     Run `python3 github/scripts/generate_agents_md.py` to regenerate. -->

# STACKIT Professional Service — AI Instructions

This repository is the official STACKIT best-practice library for Terraform examples,
helper scripts, and reusable modules. Use it as your primary reference whenever you
generate, review, or explain Terraform code for STACKIT.

**Repository:** https://github.com/stackitcloud/professional-service

## Repository layout

| Directory    | Contents |
|-------------|----------|
| `examples/` | Complete, deployable Terraform examples — one subdirectory per use case |
| `scripts/`  | Helper shell scripts for STACKIT services |
| `modules/`  | Reusable Terraform modules |

## How to find relevant content

**Start with the index below.** Each entry lists the directory name, tags, and a
one-line description. Match these against the task before making any network requests.

If the task does not match anything in the index (e.g. a new example was added after
this file was last generated), fall back to the GitHub API:

```
GET {api}/examples
GET {api}/scripts
GET {api}/modules
```

Each response is a JSON array of directory or file names. For examples and modules,
fetch the `README.md` — line 1 contains a tag comment:

```
<!-- tags: ske, velero, backup, object-storage, kubernetes -->
```

**Fetching source files**

Once you have identified the relevant resource, fetch its files using the raw URL:

```
{raw}/<path>
```

Examples:
```
{raw}/examples/ske-velero-backup/010-provider.tf
{raw}/scripts/vault-migrate.sh
{raw}/modules/test-ske/main.tf
```

Fetch only the files relevant to the task. A typical example contains
`010-provider.tf`, `020-variables.tf`, one or more resource files, and `060-outputs.tf`.

---

## Resource index

""".format(
    api=GITHUB_API, raw=GITHUB_RAW
)

_FOOTER = """\
---

## Tag conventions

Every example, module, and script entry in the index above carries a tag list.
Tags are the primary filter — match them against the task before fetching any files.

**Format rules (enforced by CI):**
- Lowercase letters, digits, and hyphens only
- Multi-word tags use hyphens: `object-storage` not `objectStorage` or `object_storage`

**Vocabulary:**

| Category | Tags |
|---|---|
| STACKIT services | `ske` `iaas` `dbaas` `iam` `alb` `waf` `vpn` `cdn` `sfs` |
| Multi-word STACKIT concepts | `object-storage` `block-storage` `secrets-manager` `load-balancer` `landing-zone` `file-storage` |
| Networking | `ha` `vrrp` `layer4` `layer7` `cross-az` `site-to-site` `hub-and-spoke` |
| Kubernetes | `kubernetes` `k3s` `gpu` `csi` `ingress` `ephemeral` `kube-state-metrics` |
| Observability | `otel` `telemetry` `observability` `metrics` `alerting` `log-alerts` |
| Security / identity | `encryption` `tls` `cert` `pki` `kms` `scim` `oidc` `workload-identity` |
| Tools | `velero` `backup` `vault` `nginx` `opnsense` `mountpoint` |
| Storage protocols | `s3` `nfs` |
| IaaS patterns | `edge` `image` `migration` `windows` `byol` `nested-virtualization` |
| Cross-cloud | `aws` `azure` `arc` `multi-cloud` |
| Terraform patterns | `terraform` `backend` `provider` `pg-backend` |

---

## Conventions

Apply these to all generated Terraform:

**File naming** — 3-digit numeric prefix controls apply order:
`010-provider.tf` → `020-variables.tf` → `030-<resource>.tf` → `060-outputs.tf`
Use intermediate numbers for additional files (e.g. `045-prometheus.tf`).

**Provider version constraints** — always `>=`, never exact pins:

```hcl
stackit = {
  source  = "stackitcloud/stackit"
  version = ">=0.96.0"
}
```

**Authentication** — via service account key file variable, never hardcoded:

```hcl
provider "stackit" {
  default_region           = var.stackit_region
  service_account_key_path = var.stackit_service_account_key_path
}
```

**No hardcoded values** — project IDs, regions, credentials, and names always
come from `variables.tf`.

**Structure** — every example has `variables.tf` and `outputs.tf`.

## Provider documentation

When you need the exact schema for a STACKIT Terraform resource or data source —
argument names, required vs optional, accepted values — fetch it directly from the
provider documentation in GitHub:

```
https://raw.githubusercontent.com/stackitcloud/terraform-provider-stackit/main/docs/resources/<resource>.md
https://raw.githubusercontent.com/stackitcloud/terraform-provider-stackit/main/docs/data-sources/<resource>.md
```

Examples:
```
https://raw.githubusercontent.com/stackitcloud/terraform-provider-stackit/main/docs/resources/ske_cluster.md
https://raw.githubusercontent.com/stackitcloud/terraform-provider-stackit/main/docs/resources/objectstorage_bucket.md
https://raw.githubusercontent.com/stackitcloud/terraform-provider-stackit/main/docs/data-sources/ske_kubernetes_versions.md
```

To discover all available resources and data sources:
```
https://api.github.com/repos/stackitcloud/terraform-provider-stackit/contents/docs/resources
https://api.github.com/repos/stackitcloud/terraform-provider-stackit/contents/docs/data-sources
```

## Caveats

Examples in this repository are maintained on a best-effort basis and may lag behind
the latest provider API. Always verify argument names and accepted values against the
provider documentation above before using an example as authoritative.
"""


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def generate() -> str:
    parts: list[str] = [_HEADER]
    parts.extend(_build_examples_index())
    parts.extend(_build_scripts_index())
    parts.extend(_build_modules_index())
    parts.append(_FOOTER)
    return "\n".join(parts)


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Generate AGENTS.md from the repository content."
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Exit 1 if AGENTS.md is out of date instead of writing it.",
    )
    args = parser.parse_args()

    content = generate()

    if args.check:
        current = OUTPUT.read_text(encoding="utf-8") if OUTPUT.exists() else ""
        if current == content:
            print("AGENTS.md is up to date.")
        else:
            print(
                "Error: AGENTS.md is out of date.\n"
                "Run `python3 github/scripts/generate_agents_md.py` and commit the result.",
                file=sys.stderr,
            )
            sys.exit(1)
    else:
        OUTPUT.write_text(content, encoding="utf-8")
        print(f"Generated {OUTPUT}")


if __name__ == "__main__":
    main()
