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
import urllib.error
import urllib.request

MAX_DIFF_CHARS = 40_000

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


def get_diff(*pathspecs):
    cmd = ["git", "diff", f"origin/{BASE_REF}...HEAD", "--"] + list(pathspecs)
    try:
        out = subprocess.check_output(cmd, text=True, stderr=subprocess.DEVNULL)
        if len(out) > MAX_DIFF_CHARS:
            out = out[:MAX_DIFF_CHARS] + "\n... (diff truncated)"
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


def call_ai(system_prompt, content):
    if not content.strip():
        return "_No relevant changes to review._"
    payload = {
        "model": AI_MODEL,
        "max_tokens": 1024,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": content},
        ],
    }
    url = f"{AI_API_URL}/v1/chat/completions"
    print(sanitize(f"[AI] POST {url} model={AI_MODEL} content_len={len(content)}"))
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
        body = sanitize(exc.read().decode(errors="replace"))
        print(f"[AI] HTTP {exc.code} {exc.reason} — {body}", file=sys.stderr)
        return f"_AI call failed: HTTP {exc.code} {exc.reason}_"
    except Exception as exc:
        print(sanitize(f"[AI] Error: {exc}"), file=sys.stderr)
        return f"_AI call failed: {exc}_"


def post_comment(body):
    payload = {"body": body}
    req = urllib.request.Request(
        f"{SERVER_URL}/api/v1/repos/{REPOSITORY}/issues/{PR_NUMBER}/comments",
        data=json.dumps(payload).encode(),
        headers={
            "Authorization": f"token {FORGEJO_TOKEN}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            if resp.status not in (200, 201):
                print(f"Failed to post comment: HTTP {resp.status}", file=sys.stderr)
                sys.exit(1)
    except urllib.error.HTTPError as exc:
        print(sanitize(f"Failed to post comment: {exc}"), file=sys.stderr)
        sys.exit(1)


CHECKS = [
    (
        "📝 Spelling & Grammar",
        lambda: get_diff("*.md", "*.tf", "*.py", "*.go", "*.yaml", "*.yml"),
        """You are a technical writing reviewer. Given a git diff, check only the prose in
Markdown files, Terraform variable descriptions, inline comments, and string literals for
spelling and grammar errors. Ignore code identifiers, resource names, and technical strings.
Return a concise markdown bullet list with the file name and approximate line number for each
issue found. If there are no issues respond with exactly: ✅ No spelling or grammar issues found.""",
    ),
    (
        "🏗️ Infrastructure Changes",
        lambda: get_diff("*.tf"),
        """You are a Terraform expert. Given a git diff of Terraform files, summarize what
infrastructure will be created, modified, or deleted. Use a short bullet list grouped by
resource type. Flag any potentially destructive changes (deletions, force-replacements) with ⚠️.
Focus on what a reviewer needs to understand the blast radius of this change.""",
    ),
    (
        "🔒 Security Review",
        lambda: get_diff("*.tf", "*.yaml", "*.yml"),
        """You are a cloud security reviewer specialising in Terraform and Kubernetes. Given a
git diff, identify security issues such as: overly broad IAM roles, missing
readOnlyRootFilesystem, absent NetworkPolicies, exposed secrets, insecure defaults, or missing
resource limits. Rate each finding with 🔴 High / 🟡 Medium / 🟢 Low severity.
If no issues are found respond with exactly: ✅ No security issues found.""",
    ),
    (
        "📐 Example Consistency",
        lambda: get_diff("examples/"),
        """You are a code reviewer for a Terraform examples repository. Given a git diff that
introduces or modifies an example, check whether it follows these conventions:
- Terraform files use 3-digit numeric prefixes (010-, 020-, 030-, …)
- Each example has a README.md and a MAINTAINERS.md
- All variables have a description attribute
- Versioned Kubernetes provider resource types are used (e.g. kubernetes_namespace_v1)
- Apache 2.0 license headers are present on all .tf files
Return a bullet list of any deviations. If everything is correct respond with exactly:
✅ Example follows repository conventions.""",
    ),
    (
        "📚 README Completeness",
        lambda: get_diff("examples/", "modules/"),
        """You are a documentation reviewer. Given a git diff containing Terraform files and
README changes, check whether every new input variable and output value defined in .tf files
is mentioned in the README.md of the same example or module directory. List any that are missing.
If all are documented respond with exactly: ✅ All variables and outputs are documented.""",
    ),
    (
        "💬 Commit Messages",
        lambda: get_commit_messages(),
        """You are a code review assistant. Given a list of commit messages, flag any that are
too vague to be useful (e.g. 'fix', 'update', 'wip', 'changes', single words with no context).
For each vague message suggest a more descriptive alternative.
If all messages are sufficiently descriptive respond with exactly: ✅ Commit messages are descriptive.""",
    ),
]


def main():
    parts = ["## 🤖 AI PR Review\n"]
    for title, get_content, system_prompt in CHECKS:
        print(f"Running check: {title}")
        result = call_ai(system_prompt, get_content())
        parts.append(f"### {title}\n\n{result}\n")
    parts.append("\n---\n_Generated automatically — treat as a hint, not a gate._")
    post_comment("\n".join(parts))
    print("Review comment posted.")


if __name__ == "__main__":
    main()
