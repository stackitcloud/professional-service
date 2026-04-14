# Contribute to the Professional Services Repository

Your contribution is welcome! Thank you for your interest in growing our shared library.

## Table of contents

- [Developer Guide](#developer-guide)
  - [Pre-Commit Checks & CI](#pre-commit-checks--ci)
  - [Repository structure](#repository-structure)
  - [Adding a new Terraform Module](#adding-a-new-terraform-module)
  - [Adding a new Script](#adding-a-new-script)
- [Code Contributions](#code-contributions)
- [Bug Reports](#bug-reports)

## Developer Guide

### Pre-Commit Checks & CI

To keep our codebase clean and secure, we enforce a strict CI pipeline on all Pull Requests. You can save time by running these checks locally before you commit:

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

- **Scan for Secrets:** Never commit credentials. We use `gitleaks` in the CI pipeline. Ensure you have no hardcoded tokens or passwords in your code.

### Repository structure

To keep things organized for everyone, please place your contributions in the correct directory:

- `modules/`: Reusable Infrastructure-as-Code modules.
- `examples/`: Working reference architectures.
- `scripts/`: Helper tools and automation scripts (Python, Bash, Go).

### Adding a new Terraform Module

If you built a great module for a customer project and want to share it, follow these steps:

1. **Create the module folder:** Create a new directory under `modules/<module_name>`.
2. **Standardize the files:** Your module should at least contain:
   - `main.tf` (The actual resources)
   - `variables.tf` (Inputs with clear descriptions and types)
   - `outputs.tf` (Values to return to the caller)
   - `README.md` (Documentation on what the module does and its inputs/outputs. We recommend using `terraform-docs` to generate this automatically).
3. **Provide an example:** A module is only as good as its documentation. Create a working example in the `examples/` folder showing how to instantiate your module.
4. **Test it locally:** Run `terraform init`, `terraform plan`, and ideally `terraform apply` in a sandbox environment to ensure your code works before opening a PR.

### Adding a new Script

When adding scripts (e.g., data migration tools, API wrappers):

1. Place it in the `scripts/` folder.
2. Include a `requirements.txt`, `go.mod`, or `package.json` if your script has external dependencies.
3. Add a short `README.md` in your script's folder explaining how to execute it and what parameters it accepts.

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
