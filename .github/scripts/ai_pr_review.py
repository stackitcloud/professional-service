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

"""AI-powered PR review that posts a consolidated comment to a Forgejo PR."""

import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

MAX_DIFF_CHARS = 40_000

_REQUIRED_ENV = [
    "AI_API_URL",
    "AI_MODEL",
    "AI_BEARER_TOKEN",
    "FORGEJO_TOKEN",
    "GITHUB_REPOSITORY",
    "PR_NUMBER",
    "GITHUB_SERVER_URL",
    "BASE_REF",
]


def validate_env():
    missing = [v for v in _REQUIRED_ENV if not os.environ.get(v)]
    if missing:
        print(
            f"Missing required environment variables: {', '.join(missing)}",
            file=sys.stderr,
        )
        sys.exit(1)


validate_env()

AI_API_URL = os.environ["AI_API_URL"].rstrip("/")
AI_MODEL = os.environ["AI_MODEL"]
AI_TOKEN = os.environ["AI_BEARER_TOKEN"]
FORGEJO_TOKEN = os.environ["FORGEJO_TOKEN"]
REPOSITORY = os.environ["GITHUB_REPOSITORY"]
PR_NUMBER = os.environ["PR_NUMBER"]
SERVER_URL = os.environ["GITHUB_SERVER_URL"].rstrip("/")
BASE_REF = os.environ["BASE_REF"]

# Secrets that must never appear in log output.
_SECRETS = [AI_TOKEN, FORGEJO_TOKEN]


def sanitize(text: str) -> str:
    """Replace any secret value in text with *** before logging."""
    for secret in _SECRETS:
        if secret:
            text = text.replace(secret, "***")
    return text


def check_git_history():
    """Ensure we have the necessary git history to perform a diff."""
    try:
        subprocess.check_call(
            ["git", "merge-base", f"origin/{BASE_REF}", "HEAD"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
    except subprocess.CalledProcessError:
        print(
            f"Error: Could not find merge base for origin/{BASE_REF} and HEAD.",
            file=sys.stderr,
        )
        print(
            "Did you configure your checkout action with fetch-depth: 0?",
            file=sys.stderr,
        )
        sys.exit(1)


def get_head_sha() -> str:
    try:
        return subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    except subprocess.CalledProcessError:
        return "unknown"


def get_diff(*pathspecs):
    cmd = ["git", "diff", f"origin/{BASE_REF}...HEAD", "--"] + list(pathspecs)
    try:
        out = subprocess.check_output(cmd, text=True, stderr=subprocess.DEVNULL)
        if len(out) > MAX_DIFF_CHARS:
            # Smart truncation: Find the last newline before the cutoff limit
            cutoff = out.rfind("\n", 0, MAX_DIFF_CHARS)
            if cutoff == -1:
                cutoff = MAX_DIFF_CHARS
            out = out[:cutoff] + "\n\n... (diff truncated due to size limits)"
        return out
    except subprocess.CalledProcessError:
        return ""


def get_commit_messages():
    try:
        return subprocess.check_output(
            ["git", "log", f"origin/{BASE_REF}..HEAD", "--format=- %s%n%b"],
            text=True,
            stderr=subprocess.DEVNULL,
        ).strip()
    except subprocess.CalledProcessError:
        return ""


# General brand rule applied to every check.
_BRAND_NOTE = "Brand note: the correct spelling is STACKIT (all caps). Flag any occurrence of StackIT, Stackit, stackit, or any other variant as a branding error.\n"

# Used for checks where only newly added lines are relevant (spelling, security, consistency).
_SCOPE_ADDITIONS = (
    "You are reviewing a pull request diff. "
    "Only flag issues on lines that are additions in the diff (lines starting with `+`). "
    "Do not report issues on context lines (starting with a space) or removed lines (starting with `-`).\n"
    + _BRAND_NOTE
    + "Respond ONLY with the requested bullet list or the exact success phrase. "
    "Do not include introductory text, explanations, or preambles.\n\n"
)

# Used for checks that need to understand the full scope of change (e.g. what
# was added AND what was removed to summarise the infrastructure delta).
_SCOPE_CHANGES = (
    "You are reviewing a pull request diff. "
    "Consider both added lines (starting with `+`) and removed lines (starting with `-`) "
    "to understand the full scope of the change. Ignore context lines (starting with a space).\n"
    + _BRAND_NOTE
    + "Respond ONLY with the requested bullet list or the exact success phrase. "
    "Do not include introductory text, explanations, or preambles.\n\n"
)


def call_ai(system_prompt, content, retries=3, additions_only=True):
    if not content.strip():
        return "_No relevant changes to review._"

    scope = _SCOPE_ADDITIONS if additions_only else _SCOPE_CHANGES
    payload = {
        "model": AI_MODEL,
        "max_tokens": 1024,
        "messages": [
            {"role": "system", "content": scope + system_prompt},
            {"role": "user", "content": content},
        ],
    }
    url = f"{AI_API_URL}/v1/chat/completions"

    for attempt in range(retries):
        if attempt == 0:
            print(
                sanitize(f"[AI] POST {url} model={AI_MODEL} content_len={len(content)}")
            )

        req = urllib.request.Request(
            url,
            data=json.dumps(payload).encode(),
            headers={
                "Authorization": f"Bearer {AI_TOKEN}",
                "Content-Type": "application/json",
            },
        )

        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                raw = resp.read()
                print(f"[AI] HTTP {resp.status} response_len={len(raw)}")
                data = json.loads(raw)
            return data["choices"][0]["message"]["content"].strip()

        except urllib.error.HTTPError as exc:
            # Exponential backoff for rate limits and server errors
            if exc.code in (429, 500, 502, 503, 504) and attempt < retries - 1:
                sleep_time = 2**attempt
                print(
                    f"[AI] HTTP {exc.code} - Retrying in {sleep_time}s...",
                    file=sys.stderr,
                )
                time.sleep(sleep_time)
                continue

            body = sanitize(exc.read().decode(errors="replace"))
            print(f"[AI] HTTP {exc.code} {exc.reason} — {body}", file=sys.stderr)
            return f"_AI call failed: HTTP {exc.code} {exc.reason}_"

        except Exception as exc:
            if attempt < retries - 1:
                sleep_time = 2**attempt
                print(
                    f"[AI] Network error: {exc} - Retrying in {sleep_time}s...",
                    file=sys.stderr,
                )
                time.sleep(sleep_time)
                continue

            print(sanitize(f"[AI] Error: {exc}"), file=sys.stderr)
            return f"_AI call failed: {exc}_"


def post_comment(body: str):
    req = urllib.request.Request(
        f"{SERVER_URL}/api/v1/repos/{REPOSITORY}/issues/{PR_NUMBER}/comments",
        data=json.dumps({"body": body}).encode(),
        headers={
            "Authorization": f"token {FORGEJO_TOKEN}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            if resp.status not in (200, 201):
                print(f"[Forgejo] Unexpected status {resp.status}", file=sys.stderr)
                sys.exit(1)
    except urllib.error.HTTPError as exc:
        print(sanitize(f"[Forgejo] Failed to post comment: {exc}"), file=sys.stderr)
        sys.exit(1)


# Each entry: (title, content_fn, system_prompt, additions_only)
# additions_only=True  → model only considers `+` lines (spelling, security, …)
# additions_only=False → model considers both `+` and `-` lines (infrastructure delta)
CHECKS = [
    (
        "📝 Spelling & Grammar",
        lambda: get_diff("*.md", "*.tf", "*.py", "*.go", "*.yaml", "*.yml"),
        """Given a git diff, check only the prose in Markdown files, Terraform variable
descriptions, inline comments, and string literals for spelling and grammar errors.
Ignore code identifiers, resource names, and technical strings.
Return a concise markdown bullet list with the file name and line number for each issue.
If there are no issues respond with exactly: ✅ No spelling or grammar issues found.""",
        True,
    ),
    (
        "🏗️ Infrastructure Changes",
        lambda: get_diff("*.tf", "*.py", "*.go", "*.yaml", "*.yml"),
        """Given a git diff of Terraform files, summarize what infrastructure will be created,
modified, or deleted. Use a short bullet list grouped by resource type.
Flag any potentially destructive changes (deletions, force-replacements) with ⚠️.
For destructive changes, suggest a safer alternative formatted as a markdown code block:
```hcl
# or ```go,py for other languages
# safer alternative here
```
Focus on what a reviewer needs to understand the blast radius of this change.""",
        False,
    ),
    (
        "🔒 Security Review",
        lambda: get_diff("*.tf", "*.yaml", "*.yml"),
        """Given a git diff, identify security issues introduced by the new lines such as:
overly broad IAM roles, missing readOnlyRootFilesystem, absent NetworkPolicies, exposed
secrets, insecure defaults, or missing resource limits.
Rate each finding with 🔴 High / 🟡 Medium / 🟢 Low severity.
For each finding include a fix formatted as a markdown code block:
```hcl
# or ```yaml for Kubernetes manifests
```
If no issues are found respond with exactly: ✅ No security issues found.""",
        True,
    ),
    (
        "📐 Example Consistency",
        lambda: get_diff("examples/"),
        """Given a git diff introducing or modifying a Terraform example, check whether the
added lines follow these conventions:
- Terraform files use 3-digit numeric prefixes (000-, 010-, 020-, 030-, …)
- Each example has a README.md and a MAINTAINERS.md
- All variables have a description attribute
- All providers in required_providers blocks have an explicit version constraint (e.g. version = ">=0.96.0")
- A .terraform.lock.hcl file is present and committed for new examples
- Apache 2.0 license headers are present on all .tf files
For each deviation provide a short inline fix as a markdown code block:
```terraform
# corrected snippet here
```
If everything is correct respond with exactly: ✅ Example follows repository conventions.""",
        True,
    ),
    (
        "📚 Example README",
        lambda: get_diff("examples/"),
        """Given a git diff, for any new or modified example under examples/ check:
1. Naming: the directory name should be kebab-case, clearly describe the technology or
   use-case (e.g. ske-velero-backup, iam-custom-roles), and not be overly generic
   (e.g. 'test', 'example', 'demo'). Flag names that are ambiguous or misleading given
   the contents of the diff.
2. README presence and clarity: a README.md must exist and include at minimum an overview
   section explaining what the example demonstrates and a usage section showing how to run
   it (terraform init / apply). Flag if missing, empty, or too sparse.
If everything is in order respond with exactly: ✅ Example READMEs are complete.""",
        True,
    ),
    (
        "📚 Module Variable & Output Coverage",
        lambda: get_diff("modules/"),
        """Given a git diff of files under modules/, check:
1. Naming: the module directory name should be kebab-case and clearly describe what the
   module provisions or abstracts (e.g. test-ske, network-base). Flag names that are
   ambiguous, too generic, or inconsistent with the module's purpose as shown in the diff.
2. README and coverage: verify the README.md exists with an overview and usage section,
   and that every new input variable and output value added in .tf files is documented in
   the README.md of the same module directory.
List all issues found. If everything is in order respond with exactly:
✅ All module variables and outputs are documented.""",
        True,
    ),
    (
        "💬 Commit Messages",
        lambda: get_commit_messages(),
        """Given a list of commit messages for this pull request, flag any that are too vague
to be useful (e.g. 'fix', 'update', 'wip', 'changes', single words with no context).
For each vague message suggest a more descriptive alternative.
If all messages are sufficiently descriptive respond with exactly:
✅ Commit messages are descriptive.""",
        True,
    ),
]


def main():
    check_git_history()

    sha = get_head_sha()
    commit_url = f"{SERVER_URL}/{REPOSITORY}/commit/{sha}"
    short_sha = sha[:8] if sha != "unknown" else sha

    parts = [
        f"## 🤖 AI PR Review\n",
        f"> Reviewing changes up to [`{short_sha}`]({commit_url})\n",
    ]

    # Run all checks in parallel to reduce total wall-clock time.
    results = [None] * len(CHECKS)
    with ThreadPoolExecutor(max_workers=len(CHECKS)) as executor:
        future_to_idx = {
            executor.submit(
                call_ai, system_prompt, get_content(), additions_only=additions_only
            ): idx
            for idx, (_, get_content, system_prompt, additions_only) in enumerate(
                CHECKS
            )
        }
        for future in as_completed(future_to_idx):
            idx = future_to_idx[future]
            title = CHECKS[idx][0]
            print(f"Completed check: {title}")
            results[idx] = (title, future.result())

    for title, result in results:
        parts.append(
            f"<details>\n<summary>{title}</summary>\n\n{result}\n\n</details>\n"
        )

    parts.append("\n---\n_Generated automatically — treat as a hint, not a gate._")
    post_comment("\n".join(parts))
    print("Review comment posted.")


if __name__ == "__main__":
    main()
