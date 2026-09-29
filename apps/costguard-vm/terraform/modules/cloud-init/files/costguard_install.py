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

"""Downloads the costguard-vm binary, checks it against the pinned hash and
installs it to /usr/local/bin. Run by costguard-install.service.

A failed download is retried (first try plus 3 retries, 5 minutes apart); a
hash mismatch fails at once, because retrying can't fix it. When the install
fails, a minimal message goes to the chat, because without the binary
nothing else can post. Nothing secret is printed: the output goes to the
serial console, which the IaaS API shows to anyone who may read the server.

Standard library only: python3 is on every Debian cloud image (cloud-init
needs it), so the VM installs no packages.
"""

import hashlib
import json
import os
import platform
import socket
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

CONFIG_PATH = "/etc/costguard/install.json"
TARGET_DIR = "/usr/local/bin"
BINARY = "costguard-vm"
ATTEMPTS = 4
PAUSE_SECONDS = 300
DOWNLOAD_TIMEOUT = 60
MAX_BYTES = 64 << 20
POST_ATTEMPTS = 3
POST_PAUSE_SECONDS = 10
ARCHITECTURES = {"x86_64": "amd64", "aarch64": "arm64"}


class InstallError(Exception):
    """An install failure; the message goes to the chat as the reason."""


def log(message):
    print(f"costguard-install: {message}", flush=True)


def architecture(machine=None):
    machine = machine or platform.machine()
    return ARCHITECTURES.get(machine, machine)


def file_name(version, arch):
    return f"{BINARY}_{version}_linux_{arch}"


def download_url(template, version, arch):
    """The download_url with {version} filled in, plus the file name."""
    base = template.replace("{version}", urllib.parse.quote(version, safe=""))
    return base.rstrip("/") + "/" + file_name(version, arch)


def secure_url(raw):
    """https, or http on the local machine (tests); the same rule as the
    binary uses for the webhook."""
    u = urllib.parse.urlsplit(raw)
    if u.scheme == "https":
        return bool(u.hostname)
    return u.scheme == "http" and u.hostname in ("localhost", "127.0.0.1", "::1")


def fetch(url, timeout=DOWNLOAD_TIMEOUT):
    with urllib.request.urlopen(url, timeout=timeout) as resp:
        # A redirect must not downgrade to plain http.
        if not secure_url(resp.geturl()):
            raise InstallError("the download was redirected to a non-https address")
        data = resp.read(MAX_BYTES + 1)
    if len(data) > MAX_BYTES:
        raise InstallError(f"the download is larger than {MAX_BYTES >> 20} MB")
    return data


def describe(err):
    """A short reason without URLs or response bodies."""
    if isinstance(err, urllib.error.HTTPError):
        return f"HTTP {err.code}"
    if isinstance(err, urllib.error.URLError):
        return f"{err.reason}"
    if isinstance(err, (TimeoutError, socket.timeout)):
        return "timed out"
    return f"{type(err).__name__}: {err}"


def install(
    cfg,
    target_dir=TARGET_DIR,
    machine=None,
    pause=PAUSE_SECONDS,
    sleep=time.sleep,
    get=fetch,
):
    """Downloads, verifies and installs the binary; raises InstallError."""
    version = cfg["version"]
    arch = architecture(machine)
    want = cfg["sha256"].get(arch)
    if not want:
        raise InstallError(
            f"no binary is pinned for this server's architecture ({arch})"
        )
    url = download_url(cfg["url"], version, arch)
    if not secure_url(url):
        raise InstallError("download_url must be an https address")
    name = file_name(version, arch)

    last = ""
    for attempt in range(1, ATTEMPTS + 1):
        try:
            data = get(url)
            break
        except InstallError:
            raise
        except Exception as err:  # every network error is worth a retry
            last = describe(err)
            log(f"downloading {name} failed (attempt {attempt} of {ATTEMPTS}): {last}")
            if attempt < ATTEMPTS:
                sleep(pause)
    else:
        raise InstallError(
            f"downloading {name} failed {ATTEMPTS} times, last error: {last}"
        )

    got = hashlib.sha256(data).hexdigest()
    if got != want.lower():
        raise InstallError(
            f"{name} does not match the pinned SHA-256 hash (got {got}), so it was not installed; "
            "the file was changed or belongs to another version"
        )

    target = os.path.join(target_dir, BINARY)
    tmp = target + ".new"
    with open(tmp, "wb") as f:
        f.write(data)
        f.flush()
        os.fsync(f.fileno())
    os.chmod(tmp, 0o755)
    os.replace(tmp, target)
    log(f"installed {name} (sha256 {got})")


def payload(output, text):
    """The minimal message in the chat's format."""
    if output == "teams":
        return {
            "type": "message",
            "attachments": [
                {
                    "contentType": "application/vnd.microsoft.card.adaptive",
                    "contentUrl": None,
                    "content": {
                        "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
                        "type": "AdaptiveCard",
                        "version": "1.4",
                        "body": [{"type": "TextBlock", "text": text, "wrap": True}],
                    },
                }
            ],
        }
    return {"text": text}  # slack and googlechat


def notify(output, webhook, text, sleep=time.sleep, opener=urllib.request.urlopen):
    """Posts text to the chat; best effort, never raises. The webhook URL is
    a secret, so it never appears in the log."""
    if not webhook or not secure_url(webhook):
        log("no usable webhook URL, so the failure is not posted to the chat")
        return False
    body = json.dumps(payload(output, text)).encode()
    for attempt in range(1, POST_ATTEMPTS + 1):
        req = urllib.request.Request(
            webhook, data=body, headers={"Content-Type": "application/json"}
        )
        try:
            with opener(req, timeout=30):
                return True
        except Exception as err:
            log(
                f"posting the failure to the chat failed (attempt {attempt} of {POST_ATTEMPTS}): {describe(err)}"
            )
            if attempt < POST_ATTEMPTS:
                sleep(POST_PAUSE_SECONDS)
    return False


def failure_text(version, reason):
    return (
        f"costguard-vm {version} could not be installed on server {socket.gethostname()}: {reason}. "
        "Nothing runs until this is fixed. Check download_url and the version in the Terraform code, "
        "then run terraform apply -replace=stackit_server.costguard."
    )


def main():
    try:
        with open(CONFIG_PATH, encoding="utf-8") as f:
            cfg = json.load(f)
    except (OSError, ValueError) as err:
        log(f"cannot read {CONFIG_PATH}: {err}")
        return 1
    try:
        install(cfg)
        return 0
    except InstallError as err:
        reason = str(err)
    except Exception as err:
        reason = f"unexpected error: {type(err).__name__}: {err}"
    log(f"install failed: {reason}")
    notify(
        cfg.get("output", ""),
        os.environ.get("COSTGUARD_WEBHOOK_URL", ""),
        failure_text(cfg["version"], reason),
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
