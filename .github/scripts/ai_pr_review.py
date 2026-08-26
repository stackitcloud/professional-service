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

"""AI-powered PR review using STACKIT Model Serving and STACKIT Cloud Advisor.

Both backends run in parallel for every check. Results are combined into a single
consolidated comment on the Forgejo PR, with each backend's output clearly labelled.
"""

import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

MAX_DIFF_CHARS = 40_000

# ---------------------------------------------------------------------------
# Environment
# ---------------------------------------------------------------------------

_REQUIRED_ENV = [
    "AI_API_URL",
    "AI_MODEL",
    "AI_BEARER_TOKEN",
    "CLOUDMENT_TOKEN",
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
CLOUDMENT_TOKEN = os.environ["CLOUDMENT_TOKEN"]
CLOUDMENT_API_URL = "https://app.essential.cloudment.io"
FORGEJO_TOKEN = os.environ["FORGEJO_TOKEN"]
REPOSITORY = os.environ["GITHUB_REPOSITORY"]
PR_NUMBER = os.environ["PR_NUMBER"]
SERVER_URL = os.environ["GITHUB_SERVER_URL"].rstrip("/")
BASE_REF = os.environ["BASE_REF"]

# Secrets that must never appear in log output.
_SECRETS = [AI_TOKEN, CLOUDMENT_TOKEN, FORGEJO_TOKEN]


def sanitize(text: str) -> str:
    """Replace any secret value in text with *** before logging."""
    for secret in _SECRETS:
        if secret:
            text = text.replace(secret, "***")
    return text


# ---------------------------------------------------------------------------
# Git helpers
# ---------------------------------------------------------------------------


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
            # Smart truncation: find the last newline before the cutoff limit.
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


# ---------------------------------------------------------------------------
# STACKIT Model Serving client (OpenAI-compatible)
# ---------------------------------------------------------------------------

# General brand rule applied to every LLM check.
_BRAND_NOTE = (
    "Brand note: the correct and only acceptable spelling is STACKIT (all caps). "
    "Do NOT flag STACKIT — it is already correct. Only flag incorrect variants such as "
    "StackIT, Stackit, stackit, Stack IT, or any other casing or spacing variant.\n"
)

# Used for checks where only newly added lines are relevant (spelling, security, consistency).
_SCOPE_ADDITIONS = (
    "You are reviewing a pull request diff. "
    "Only flag issues on lines that are additions in the diff (lines starting with `+`). "
    "Do not report issues on context lines (starting with a space) or removed lines (starting with `-`).\n"
    + _BRAND_NOTE
    + "Respond ONLY with the requested bullet list or the exact success phrase. "
    "Do not include introductory text, explanations, or preambles.\n\n"
)

# Used for checks that need to understand the full scope of change (e.g. what was added
# AND what was removed to summarise the infrastructure delta).
_SCOPE_CHANGES = (
    "You are reviewing a pull request diff. "
    "Consider both added lines (starting with `+`) and removed lines (starting with `-`) "
    "to understand the full scope of the change. Ignore context lines (starting with a space).\n"
    + _BRAND_NOTE
    + "Respond ONLY with the requested bullet list or the exact success phrase. "
    "Do not include introductory text, explanations, or preambles.\n\n"
)


def call_llm(
    system_prompt: str,
    content: str,
    retries: int = 3,
    additions_only: bool = True,
) -> str:
    """Call the STACKIT Model Serving (OpenAI-compatible chat completions)."""
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
                sanitize(
                    f"[LLM] POST {url} model={AI_MODEL} content_len={len(content)}"
                )
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
                print(f"[LLM] HTTP {resp.status} response_len={len(raw)}")
                data = json.loads(raw)
            return data["choices"][0]["message"]["content"].strip()

        except urllib.error.HTTPError as exc:
            if exc.code in (429, 500, 502, 503, 504) and attempt < retries - 1:
                sleep_time = 2**attempt
                print(
                    f"[LLM] HTTP {exc.code} - Retrying in {sleep_time}s...",
                    file=sys.stderr,
                )
                time.sleep(sleep_time)
                continue
            body = sanitize(exc.read().decode(errors="replace"))
            print(f"[LLM] HTTP {exc.code} {exc.reason} — {body}", file=sys.stderr)
            return f"_LLM call failed: HTTP {exc.code} {exc.reason}_"

        except Exception as exc:
            if attempt < retries - 1:
                sleep_time = 2**attempt
                print(
                    f"[LLM] Network error: {exc} - Retrying in {sleep_time}s...",
                    file=sys.stderr,
                )
                time.sleep(sleep_time)
                continue
            print(sanitize(f"[LLM] Error: {exc}"), file=sys.stderr)
            return f"_LLM call failed: {exc}_"

    return f"_LLM call failed: exhausted {retries} retries._"


# ---------------------------------------------------------------------------
# STACKIT Cloud Advisor client (Cloudment)
# ---------------------------------------------------------------------------


def call_advisor(question: str, retries: int = 3) -> str:
    """Call the Cloudment STACKIT Cloud Advisor and return the formatted answer.

    The API is stateless (one question per request) and can take several minutes
    for complex multi-product questions; the timeout is set to 600 s accordingly.
    Citations returned by the API are appended as a source list.
    """
    if not question.strip():
        return "_No relevant changes to review._"

    payload = {"message": question}
    url = f"{CLOUDMENT_API_URL}/v1/messages"

    for attempt in range(retries):
        if attempt == 0:
            print(sanitize(f"[Advisor] POST {url} question_len={len(question)}"))

        req = urllib.request.Request(
            url,
            data=json.dumps(payload).encode(),
            headers={
                "Authorization": f"Bearer {CLOUDMENT_TOKEN}",
                "Content-Type": "application/json",
            },
        )

        try:
            with urllib.request.urlopen(req, timeout=600) as resp:
                raw = resp.read()
                print(f"[Advisor] HTTP {resp.status} response_len={len(raw)}")
                data = json.loads(raw)

            if data.get("blocked"):
                reason = data.get("blocked_reason") or "No reason provided."
                return f"_Advisor blocked this question: {reason}_"

            answer = (data.get("answer") or "").strip()
            citations = data.get("citations") or []
            if citations:
                links = "\n".join(f"- <{c['url']}>" for c in citations if c.get("url"))
                answer += f"\n\n**Sources:**\n{links}"
            return answer or "_Advisor returned an empty answer._"

        except urllib.error.HTTPError as exc:
            if exc.code in (502, 503, 504) and attempt < retries - 1:
                sleep_time = 2**attempt
                print(
                    f"[Advisor] HTTP {exc.code} - Retrying in {sleep_time}s...",
                    file=sys.stderr,
                )
                time.sleep(sleep_time)
                continue
            body = sanitize(exc.read().decode(errors="replace"))
            print(f"[Advisor] HTTP {exc.code} {exc.reason} — {body}", file=sys.stderr)
            return f"_Advisor call failed: HTTP {exc.code} {exc.reason}_"

        except Exception as exc:
            if attempt < retries - 1:
                sleep_time = 2**attempt
                print(
                    f"[Advisor] Network error: {exc} - Retrying in {sleep_time}s...",
                    file=sys.stderr,
                )
                time.sleep(sleep_time)
                continue
            print(sanitize(f"[Advisor] Error: {exc}"), file=sys.stderr)
            return f"_Advisor call failed: {exc}_"

    return f"_Advisor call failed: exhausted {retries} retries._"


# ---------------------------------------------------------------------------
# Forgejo comment
# ---------------------------------------------------------------------------


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


# ---------------------------------------------------------------------------
# Checks
# ---------------------------------------------------------------------------
#
# Each check is a dict with:
#   title        – Display heading in the PR comment.
#   content_fn   – Callable → str; the diff or text to analyse.
#   llm          – dict:
#                    system_prompt   instructions for STACKIT Model Serving.
#                    additions_only  True = only `+` lines; False = `+` and `-`.
#   advisor      – dict:
#                    question_fn     Callable(content: str) → str; builds the
#                                    natural-language question for STACKIT Cloud Advisor.
#
# Both backends run in parallel for every check. Results are rendered under
# separate "STACKIT Model Serving" / "STACKIT Cloud Advisor" sub-headings.

CHECKS = [
    {
        "title": "📝 Spelling & Grammar",
        "content_fn": lambda: get_diff(
            "*.md", "*.tf", "*.py", "*.go", "*.yaml", "*.yml"
        ),
        "llm": {
            "system_prompt": (
                "Given a git diff, check only the prose in Markdown files, Terraform variable "
                "descriptions, inline comments, and string literals for spelling and grammar errors. "
                "Ignore code identifiers, resource names, URLs, and technical strings. "
                "Only report lines that contain an actual error — do NOT list lines that are already "
                "correct. Return a concise markdown bullet list with file name and line number for "
                "each error, quoting the wrong text and the correction. "
                "If there are no errors respond with exactly: ✅ No spelling or grammar issues found."
            ),
            "additions_only": True,
        },
        "advisor": {
            "question_fn": lambda content: (
                "Review the following git diff for spelling and grammar errors in prose content "
                "(Markdown, Terraform variable descriptions, inline comments, string literals). "
                "Ignore code identifiers, resource names, URLs, and technical strings. "
                "Pay special attention to STACKIT product names and terminology. "
                "Report ONLY lines that contain an actual error — do NOT mention lines that are "
                "already correct. For each error state the file, line number, wrong text, and "
                "correction. "
                "If there are no errors respond with exactly: ✅ No spelling or grammar issues found.\n\n"
                f"Git diff:\n```\n{content}\n```"
            ),
        },
    },
    {
        "title": "🏗️ Infrastructure Changes",
        "content_fn": lambda: get_diff("*.tf", "*.py", "*.go", "*.yaml", "*.yml"),
        "llm": {
            "system_prompt": (
                "Given a git diff of Terraform files, summarize what infrastructure will be created, "
                "modified, or deleted. Use a short bullet list grouped by resource type. "
                "Flag any potentially destructive changes (deletions, force-replacements) with ⚠️. "
                "For destructive changes, suggest a safer alternative formatted as a markdown code block:\n"
                "```hcl\n# or ```go,py for other languages\n# safer alternative here\n```\n"
                "Focus on what a reviewer needs to understand the blast radius of this change."
            ),
            "additions_only": False,
        },
        "advisor": {
            "question_fn": lambda content: (
                "I am reviewing a pull request that modifies infrastructure on STACKIT. "
                "Based on the following git diff, please:\n"
                "1. Identify which STACKIT services (e.g. SKE, Object Storage, DNS, Load Balancer, "
                "PostgreSQL Flex, etc.) are being provisioned, changed, or removed.\n"
                "2. Highlight any STACKIT-specific best practices that are not being followed.\n"
                "3. Flag STACKIT service quotas, regional constraints, or known limitations that "
                "may affect this change.\n\n"
                f"Git diff:\n```\n{content}\n```"
            ),
        },
    },
    {
        "title": "🔒 Security Review",
        "content_fn": lambda: get_diff("*.tf", "*.yaml", "*.yml"),
        "llm": {
            "system_prompt": (
                "Given a git diff, identify security issues introduced by the new lines such as: "
                "overly broad IAM roles, missing readOnlyRootFilesystem, absent NetworkPolicies, exposed "
                "secrets, insecure defaults, or missing resource limits. "
                "Rate each finding with 🔴 High / 🟡 Medium / 🟢 Low severity. "
                "For each finding include a fix formatted as a markdown code block:\n"
                "```hcl\n# or ```yaml for Kubernetes manifests\n```\n"
                "If no issues are found respond with exactly: ✅ No security issues found."
            ),
            "additions_only": True,
        },
        "advisor": {
            "question_fn": lambda content: (
                "I am reviewing a pull request that changes infrastructure deployed on STACKIT. "
                "Based on the following git diff, please identify:\n"
                "1. STACKIT IAM or authorization misconfigurations (e.g. overly permissive service "
                "account roles or project member assignments).\n"
                "2. Missing STACKIT-specific security controls (e.g. private endpoint usage, "
                "network policy gaps for SKE, absent audit logging).\n"
                "3. Any secrets, credentials, or sensitive values that appear to be hard-coded.\n"
                "4. STACKIT compliance or audit-logging considerations relevant to these changes.\n"
                "Rate each finding as 🔴 High, 🟡 Medium, or 🟢 Low.\n"
                "If no issues are found respond with: ✅ No STACKIT-specific security issues found.\n\n"
                f"Git diff:\n```\n{content}\n```"
            ),
        },
    },
    {
        "title": "📐 Example Consistency",
        "content_fn": lambda: get_diff("examples/"),
        "llm": {
            "system_prompt": (
                "Given a git diff introducing or modifying a Terraform example, check whether the "
                "added lines follow these conventions:\n"
                "- Terraform files use 3-digit numeric prefixes (000-, 010-, 020-, 030-, …)\n"
                "- Each example has a README.md and a MAINTAINERS.md\n"
                "- All variables have a description attribute\n"
                "- All variable names use snake_case (e.g. my_variable, not myVariable, my-variable, "
                "or MyVariable)\n"
                "- All providers in required_providers blocks have an explicit version constraint "
                '(e.g. version = ">=0.96.0")\n'
                "- A .terraform.lock.hcl file is present and committed for new examples\n"
                "- Apache 2.0 license headers are present on all .tf files\n"
                "For each deviation provide a short inline fix as a markdown code block:\n"
                "```terraform\n# corrected snippet here\n```\n"
                "If everything is correct respond with exactly: "
                "✅ Example follows repository conventions."
            ),
            "additions_only": True,
        },
        "advisor": {
            "question_fn": lambda content: (
                "Review the following git diff of a STACKIT Terraform example for repository conventions:\n"
                "- Terraform files use 3-digit numeric prefixes (000-, 010-, 020-, 030-, …)\n"
                "- Each example has a README.md and a MAINTAINERS.md\n"
                "- All variables have a description attribute and use snake_case naming\n"
                "- All providers in required_providers blocks have an explicit version constraint\n"
                "- A .terraform.lock.hcl file is present and committed for new examples\n"
                "- Apache 2.0 license headers are present on all .tf files\n"
                "Also flag any STACKIT-specific conventions or provider usage concerns. "
                "For each deviation provide a short fix as a markdown code block. "
                "If everything is correct respond with exactly: "
                "✅ Example follows repository conventions.\n\n"
                f"Git diff:\n```\n{content}\n```"
            ),
        },
    },
    {
        "title": "📚 Example README",
        "content_fn": lambda: get_diff("examples/"),
        "llm": {
            "system_prompt": (
                "Given a git diff, for any new or modified example under examples/ check:\n"
                "1. Naming: the directory name should be kebab-case, clearly describe the technology "
                "or use-case (e.g. ske-velero-backup, iam-custom-roles), and not be overly generic "
                "(e.g. 'test', 'example', 'demo'). Flag names that are ambiguous or misleading given "
                "the contents of the diff.\n"
                "2. README presence and clarity: a README.md must exist and include at minimum an "
                "overview section explaining what the example demonstrates and a usage section showing "
                "how to run it (terraform init / apply). Flag if missing, empty, or too sparse.\n"
                "If everything is in order respond with exactly: ✅ Example READMEs are complete."
            ),
            "additions_only": True,
        },
        "advisor": {
            "question_fn": lambda content: (
                "Review the following git diff for any new or modified STACKIT example under examples/:\n"
                "1. Naming: does the directory name clearly and accurately describe the STACKIT service "
                "or use-case being demonstrated (e.g. ske-velero-backup, objectstorage-lifecycle)? "
                "Flag names that are ambiguous, too generic, or inconsistent with the diff content.\n"
                "2. README quality: does the README.md explain what STACKIT services are used, "
                "what the example demonstrates, and include a usage section (terraform init / apply)? "
                "Flag if missing, empty, or too sparse.\n"
                "If everything is in order respond with exactly: ✅ Example READMEs are complete.\n\n"
                f"Git diff:\n```\n{content}\n```"
            ),
        },
    },
    {
        "title": "📚 Module Variable & Output Coverage",
        "content_fn": lambda: get_diff("modules/"),
        "llm": {
            "system_prompt": (
                "Given a git diff of files under modules/, check:\n"
                "1. Naming: the module directory name should be kebab-case and clearly describe what "
                "the module provisions or abstracts (e.g. test-ske, network-base). Flag names that "
                "are ambiguous, too generic, or inconsistent with the module's purpose as shown in "
                "the diff.\n"
                "2. README and coverage: verify the README.md exists with an overview and usage "
                "section, and that every new input variable and output value added in .tf files is "
                "documented in the README.md of the same module directory.\n"
                "List all issues found. If everything is in order respond with exactly:\n"
                "✅ All module variables and outputs are documented."
            ),
            "additions_only": True,
        },
        "advisor": {
            "question_fn": lambda content: (
                "Review the following git diff of a STACKIT Terraform module under modules/:\n"
                "1. Naming: does the module directory name clearly describe what STACKIT resource or "
                "abstraction it provisions (e.g. ske-cluster, dns-zone)? Flag ambiguous or generic names.\n"
                "2. Documentation: does a README.md exist with an overview and usage section? "
                "Is every new input variable and output value in .tf files documented in the README?\n"
                "3. Are there any STACKIT-specific variables, outputs, or provider patterns that "
                "should be documented or are missing?\n"
                "List all issues. If everything is in order respond with exactly:\n"
                "✅ All module variables and outputs are documented.\n\n"
                f"Git diff:\n```\n{content}\n```"
            ),
        },
    },
    {
        "title": "💬 Commit Messages",
        "content_fn": lambda: get_commit_messages(),
        "llm": {
            "system_prompt": (
                "Given a list of commit messages for this pull request, flag any that are too vague "
                "to be useful (e.g. 'fix', 'update', 'wip', 'changes', single words with no context). "
                "For each vague message suggest a more descriptive alternative. "
                "If all messages are sufficiently descriptive respond with exactly:\n"
                "✅ Commit messages are descriptive."
            ),
            "additions_only": True,
        },
        "advisor": {
            "question_fn": lambda content: (
                "Review the following commit messages from a pull request on a STACKIT professional "
                "services repository. Flag any that are too vague to be useful (e.g. 'fix', 'update', "
                "'wip', 'changes', single words with no context). "
                "For each vague message suggest a more descriptive alternative that reflects the "
                "STACKIT service or resource being changed. "
                "If all messages are sufficiently descriptive respond with exactly:\n"
                "✅ Commit messages are descriptive.\n\n"
                f"Commit messages:\n{content}"
            ),
        },
    },
    {
        "title": "🏷️ Tag Quality",
        "content_fn": lambda: get_diff("examples/", "modules/", "scripts/"),
        "llm": {
            "system_prompt": (
                "Given a git diff of files under examples/, modules/, and scripts/, check whether "
                "the tags for any new or modified resource accurately reflect what it does.\n\n"
                "Tag locations differ by resource type:\n"
                "- examples/  → `<!-- tags: ... -->` comment on line 1 of the example's README.md\n"
                "- modules/   → `<!-- tags: ... -->` comment on line 1 of the module's README.md\n"
                "- scripts/   → Tags column in the overview table of scripts/README.md\n\n"
                "For each touched resource check:\n"
                "1. Do the tags cover the STACKIT products used? "
                "(e.g. ske, dbaas, cdn, object-storage, secretsmanager, iaas)\n"
                "2. Are key technologies or use-cases missing? "
                "(e.g. velero, nginx, cert-manager, backup, encryption, workload-identity, ha)\n"
                "3. Are all tags lowercase and hyphen-separated? "
                "(e.g. rate-limit not rateLimit or rate_limit)\n"
                "Only flag issues for resources touched in this diff. "
                "For each issue suggest the corrected tag line or table cell. "
                "If everything is correct respond with exactly: "
                "✅ README tags accurately reflect the content."
            ),
            "additions_only": False,
        },
        "advisor": {
            "question_fn": lambda content: (
                "Review the following git diff covering examples/, modules/, and scripts/ in a "
                "STACKIT professional-service repository. Check whether the tags for any new or "
                "modified resource accurately reflect what it does.\n\n"
                "Tag locations differ by resource type:\n"
                "- examples/ → `<!-- tags: ... -->` on line 1 of the example's README.md\n"
                "- modules/  → `<!-- tags: ... -->` on line 1 of the module's README.md\n"
                "- scripts/  → Tags column in the overview table of scripts/README.md\n\n"
                "For each touched resource check:\n"
                "1. Do the tags include the correct STACKIT products? "
                "(e.g. ske, dbaas, cdn, object-storage, secretsmanager, iaas)\n"
                "2. Are important open-source tools or patterns missing? "
                "(e.g. velero, nginx, cert-manager, backup, workload-identity, encryption)\n"
                "3. Are all tags lowercase and hyphen-separated?\n"
                "Only evaluate resources touched in this diff. "
                "For each issue suggest the corrected tag line or table cell. "
                "If everything is correct respond with exactly: "
                "✅ README tags accurately reflect the content.\n\n"
                f"Git diff:\n```\n{content}\n```"
            ),
        },
    },
]


# ---------------------------------------------------------------------------
# Check execution
# ---------------------------------------------------------------------------


def _run_check(check: dict) -> tuple[str, str | None, str | None]:
    """Execute one check and return (title, llm_result, advisor_result).

    Returns (title, None, None) when the content source is empty so that
    main() can skip the section entirely without displaying placeholder text.
    Both backends run in parallel when content is present.
    """
    title = check["title"]
    content = check["content_fn"]()

    if not content.strip():
        return title, None, None

    llm_cfg = check.get("llm")
    advisor_cfg = check.get("advisor")

    llm_future = None
    advisor_future = None

    with ThreadPoolExecutor(max_workers=2) as pool:
        if llm_cfg:
            llm_future = pool.submit(
                call_llm,
                llm_cfg["system_prompt"],
                content,
                additions_only=llm_cfg.get("additions_only", True),
            )
        if advisor_cfg:
            advisor_future = pool.submit(
                call_advisor,
                advisor_cfg["question_fn"](content),
            )

    llm_result = llm_future.result() if llm_future else None
    advisor_result = advisor_future.result() if advisor_future else None

    return title, llm_result, advisor_result


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def main():
    check_git_history()

    sha = get_head_sha()
    commit_url = f"{SERVER_URL}/{REPOSITORY}/commit/{sha}"
    short_sha = sha[:8] if sha != "unknown" else sha

    parts = [
        "## 🤖 AI PR Review\n",
        f"> [`{short_sha}`]({commit_url}) · STACKIT Model Serving & STACKIT Cloud Advisor\n",
    ]

    # Run all checks in parallel; within each check, LLM + Advisor also run in parallel.
    results: list[tuple[str, str | None, str | None] | None] = [None] * len(CHECKS)
    with ThreadPoolExecutor(max_workers=len(CHECKS)) as executor:
        future_to_idx = {
            executor.submit(_run_check, check): idx for idx, check in enumerate(CHECKS)
        }
        for future in as_completed(future_to_idx):
            idx = future_to_idx[future]
            title, llm_result, advisor_result = future.result()
            print(f"Completed check: {title}")
            results[idx] = (title, llm_result, advisor_result)

    for title, llm_result, advisor_result in results:
        # Skip sections where the diff/content source was empty.
        if llm_result is None and advisor_result is None:
            print(f"Skipping empty section: {title}")
            continue

        # Derive a status prefix so reviewers see pass/fail without expanding.
        # ✅ — every non-None result contains ✅ and no severity markers (🔴/🟡).
        # ❌ — any result reports a backend failure.
        # ⚠️ — everything else (issues found, or informational summary).
        all_results = [r for r in (llm_result, advisor_result) if r is not None]
        if any("call failed" in r for r in all_results):
            status = "❌ "
        elif all("✅" in r and "🔴" not in r and "🟡" not in r for r in all_results):
            status = "✅ "
        else:
            status = "⚠️ "

        inner = ""
        if llm_result is not None:
            # Guard against blank responses producing an empty details block.
            llm_body = llm_result.strip() or "_No response from STACKIT Model Serving._"
            inner += (
                "<details>\n"
                "<summary>🤖 STACKIT Model Serving</summary>\n\n"
                f"{llm_body}\n\n"
                "</details>\n\n"
            )
        if advisor_result is not None:
            advisor_body = (
                advisor_result.strip() or "_No response from STACKIT Cloud Advisor._"
            )
            inner += (
                "<details>\n"
                "<summary>🔍 STACKIT Cloud Advisor</summary>\n\n"
                f"{advisor_body}\n\n"
                "</details>\n"
            )

        parts.append(
            f"<details>\n<summary>{status}{title}</summary>\n\n{inner}\n</details>\n"
        )

    parts.append("\n---\n_Generated automatically — treat as a hint, not a gate._")
    post_comment("\n".join(parts))
    print("Review comment posted.")


if __name__ == "__main__":
    main()
