# Contribute to the Professional Services Repository

Your contribution is welcome! Thank you for your interest in growing our shared library.

## Table of contents

- [Developer Guide](#developer-guide)
  - [Pre-Commit Checks & CI](#pre-commit-checks--ci)
  - [Repository structure](#repository-structure)
  - [Adding a new Example](#adding-a-new-example)
  - [Adding a new Terraform Module](#adding-a-new-terraform-module)
  - [Adding a new Script](#adding-a-new-script)
- [Code Contributions](#code-contributions)
- [Bug Reports](#bug-reports)

## Developer Guide

### Pre-Commit Checks & CI

To maintain a clean and secure codebase, we enforce a strict CI pipeline on all Pull Requests. You can save time and catch pipeline failures early by running these checks locally before you commit your code. We use pre-commit to automate this process.

- **Format your code:** The pipeline will fail if your code is not formatted according to industry standards.
  - Terraform: `terraform fmt -recursive`
  - Python: `black .`
  - Go: `gofmt -w .`
  - JavaScript: `npx prettier --write "**/*.js"`
- **Add License Headers:** Every file must contain our Apache 2.0 license header.
  - Run: `addlicense -c "Schwarz Digits Cloud GmbH & Co. KG" -l apache .` (Requires the [google/addlicense](https://github.com/google/addlicense) tool).

```terraform
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
```

```go
// Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
```

- **Terraform file naming:** All `.tf` files in examples **must** be prefixed with exactly 3 digits to enforce consistent ordering (e.g., `010-provider.tf`, `020-variables.tf`, `030-resources.tf`, `100-outputs.tf`). Files inside `modules/` directories are exempt from this rule. This check is enforced automatically by pre-commit.
- **Tags:** Every example and module `README.md` must have a tag comment as its **very first line**. Scripts must have a Tags column entry in the `scripts/README.md` overview table. The `check-readme-tags` pre-commit hook and CI job enforce this automatically — see [the tagging rules](#tagging) below.
- **AGENTS.md is auto-generated:** Do not edit `AGENTS.md` by hand. The `generate-agents-md` pre-commit hook regenerates it automatically whenever you change an example, module, or script. The `agents-md-check` CI job fails if the file is stale.
- **No TODO comments:** The CI `todo-check` job rejects open TODO markers in code (prefixed with `#` or `//`) anywhere in the repository. Resolve all TODOs before opening a PR.
- **Scan for Secrets:** Never commit credentials. We use `gitleaks` in the CI pipeline. Ensure you have no hardcoded tokens or passwords in your code.

#### Tagging

Tags drive the resource index in `AGENTS.md` and are the primary way AI coding assistants (and humans) discover relevant examples. The `check-readme-tags` hook validates both presence and format on every commit.

**Format rules (enforced by CI):**

- Lowercase letters, digits, and hyphens only — e.g. `object-storage`, not `objectStorage` or `object_storage`
- Place the comment on **line 1** of `README.md`, before any other content

```markdown
<!-- tags: ske, velero, backup, object-storage, kubernetes -->

# My Example Title
```

**Recommended tags** (see `AGENTS.md → Tag conventions` for the full vocabulary):

| Category            | Tags                                                                              |
| ------------------- | --------------------------------------------------------------------------------- |
| STACKIT services    | `ske` `iaas` `dbaas` `iam` `alb` `waf` `vpn` `cdn` `sfs`                          |
| Multi-word concepts | `object-storage` `block-storage` `secrets-manager` `load-balancer` `landing-zone` |
| Kubernetes          | `kubernetes` `k3s` `gpu` `csi` `ingress` `ephemeral`                              |
| Observability       | `otel` `telemetry` `observability` `metrics` `alerting`                           |
| Security            | `encryption` `tls` `cert` `pki` `kms` `workload-identity`                         |
| Networking          | `ha` `vrrp` `layer4` `layer7` `ipsec` `bgp` `sna` `hub-and-spoke`                 |
| Tools               | `velero` `backup` `vault` `nginx` `opnsense`                                      |

### Repository structure

To keep things organized for everyone, please place your contributions in the correct directory:

- `modules/`: Reusable Infrastructure-as-Code modules.
- `examples/`: Working reference architectures.
- `scripts/`: Helper tools and automation scripts (Python, Bash, Go).

### Adding a new Example

Examples live in `examples/<example-name>/` and should be complete, deployable Terraform configurations.

1. **Create the example folder** using a descriptive, hyphenated name: `examples/vpn-stackit-gcp/`.
2. **Name your files** with 3-digit numeric prefixes to control apply order:
   - `010-provider.tf` — provider and `terraform` blocks
   - `020-variables.tf` — all input variables
   - `030-<resource>.tf` (or multiple: `030-network.tf`, `040-server.tf`)
   - `060-outputs.tf` — output values
3. **Add a `README.md`** with a tag comment on line 1, followed by a description, architecture overview, prerequisites, and usage:

   ```markdown
   <!-- tags: ske, velero, backup, object-storage, kubernetes -->

   # My Example Title

   ...
   ```

4. **Add a `terraform.tfvars.example`** with placeholder values for every required variable (no real IDs, keys, or credentials):

   ```hcl
   stackit_org_id                   = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
   stackit_service_account_key_path = "/path/to/stackit-sa.json"
   ```

5. **Do not commit** `terraform.tfvars`, `terraform.tfstate`, `terraform.tfstate.backup`, `.terraform.lock.hcl`, or the `.terraform/` directory.
6. **Test it locally:** `terraform init && terraform plan` must succeed before opening a PR.
7. **AGENTS.md updates automatically** — the `generate-agents-md` pre-commit hook regenerates it from your README tags. No manual edit needed.

### Adding a new Terraform Module

If you built a great module for a customer project and want to share it, follow these steps:

1. **Create the module folder:** Create a new directory under `modules/<module_name>`.
2. **Standardize the files:** Your module should at least contain:
   - `main.tf` (The actual resources)
   - `variables.tf` (Inputs with clear descriptions and types)
   - `outputs.tf` (Values to return to the caller)
   - `README.md` (Documentation on what the module does and its inputs/outputs. We recommend using `terraform-docs` to generate this automatically).
3. **Add tags on line 1** of `README.md` — see [Tagging](#tagging):

   ```markdown
   <!-- tags: ske, kubernetes, helm -->
   ```

4. **Test it locally:** Run `terraform init`, `terraform plan`, and ideally `terraform apply` in a sandbox environment to ensure your code works before opening a PR.

### Adding a new Script

When adding scripts (e.g., automation tools, API wrappers):

1. **Place it in `scripts/`.**
2. **Add a row to the overview table** in `scripts/README.md` — this is the canonical documentation for scripts (not a separate folder README). Include a Purpose, required tools, and a Tags column:

   ```markdown
   | [`my-script.sh`](#my-scriptsh) | What this script does. | `stackit`, `jq` | `ske, kubernetes` |
   ```

3. **Tags in the table must follow the same format rules** as README tags (lowercase, hyphens). The `check-readme-tags` hook validates the Tags column on every commit.
4. **Add the detailed section** for your script below the table in `scripts/README.md`, explaining usage, flags, and examples.
5. If the script has external package dependencies, include a `requirements.txt`, `go.mod`, or `package.json`.

## Code Contributions

To make your contribution, follow these steps:

1. **Check existing work:** Check open [Pull Requests] and [Issues] to make sure the contribution you are making hasn't already been tackled by someone else.
2. **Branch off:** Create a new branch from `main` (e.g., `feature/aws-eks-module` or `fix/python-script-typo`).
3. **Commit your changes:** Write descriptive commit messages. Ensure all local formatting and license checks have passed.
4. **Open a Pull Request:** Create a PR against the `main` branch.
5. **Review:** The PR will be reviewed by the repository `CODEOWNERS`. If the CI pipeline fails, please check the GitHub Actions logs and fix the formatting or secret leaks. When the PR is approved and checks pass, it will be squashed and merged.

> [!TIP]
>
> To ensure smooth review and integration of your code contributions:
>
> **Break down large changes into smaller PRs**: If you are introducing 5 new modules, consider opening 5 separate Pull Requests. This allows us to provide faster feedback and keeps the reviews manageable.
>
> **Create a draft PR for early feedback**: If you want feedback on an architecture or script during the implementation process, open a Draft PR.

## Bug Reports

Because we operate on a "Best Effort" basis, we heavily rely on you to report (and ideally fix!) bugs. If you find a module that uses deprecated APIs or a script that crashes:

1. **Fix it yourself (Preferred):** The fastest way to get a bug fixed is to submit a Pull Request with the solution.
2. **Open an Issue:** If you don't have the time to fix it, please open a GitHub issue.
3. **Be specific:** When opening an issue, provide as much context as possible:
   - Which module/script is broken?
   - What error message did you get?
   - What versions of Terraform/Python/Go are you using?
   - Include code snippets of how you called the module. This makes the reproduction process much easier.
