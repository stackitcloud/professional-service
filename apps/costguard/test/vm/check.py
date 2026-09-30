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


"""Evaluates a QEMU test boot: the collector's VMTEST lines on the serial
console and what the fake webhook received. Exit code 1 on any failure.
"""

import argparse
import json
import os
import re
import sys


def parse(serial):
    values, timers = {}, []
    for line in serial.splitlines():
        m = re.search(r"VMTEST (\w+)=(.*)$", line)
        if not m:
            continue
        key, value = m.group(1), m.group(2).strip()
        if key == "timer":
            timers.append(value)
        else:
            values[key] = value
    return values, timers


def number(value, default=99.0):
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def texts(webhook_path):
    """Every text of every posted message, one string per post."""
    posts = []
    if os.path.exists(webhook_path):
        with open(webhook_path, encoding="utf-8") as f:
            for line in f:
                posts.append(
                    json.dumps(json.loads(line)["payload"], ensure_ascii=False)
                )
    return posts


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--scenario", required=True, choices=["happy", "mismatch"])
    ap.add_argument("--serial", required=True)
    ap.add_argument("--webhook", required=True)
    ap.add_argument("--secret", required=True)
    ap.add_argument("--version", required=True)
    args = ap.parse_args()

    with open(args.serial, encoding="utf-8", errors="replace") as f:
        serial = f.read()
    v, timers = parse(serial)
    posts = texts(args.webhook)
    failures = []

    def check(ok, what):
        print(("  ok   " if ok else "  FAIL ") + what)
        if not ok:
            failures.append(what)

    print(f"check.py: scenario {args.scenario}")
    check(
        "DONE" in serial.split("VMTEST ")[-1] if "VMTEST " in serial else False,
        "the collector finished (VMTEST DONE)",
    )

    # Nothing secret on the console (the IaaS API shows it to project readers).
    check(args.secret not in serial, "the webhook URL never reaches the serial console")
    check(
        "eyJhbGciOiAiUlMyNTYi" not in serial and "eyJhbGciOiJSUzI1NiI" not in serial,
        "no token on the serial console",
    )

    check(
        "failed schema validation" not in serial,
        "cloud-init accepts the user data (schema)",
    )
    expected_failed = {
        "happy": {"costguard@boot.service", "costguard@flag.service"},
        "mismatch": {"costguard-install.service"},
    }
    failed_units = set(v.get("failed_units", "").split())
    check(
        failed_units <= expected_failed[args.scenario],
        f"no unexpected failed units ({' '.join(sorted(failed_units)) or '-'})",
    )

    # Hardening, in both scenarios.
    check(
        v.get("cloud_init") == "done",
        f"cloud-init finished without errors ({v.get('cloud_init')})",
    )
    check(v.get("ssh") == "masked/masked", f"sshd is masked ({v.get('ssh')})")
    check(
        v.get("ssh_unix") == "masked/inactive/0",
        f"no local sshd socket listening ({v.get('ssh_unix')})",
    )
    check(
        v.get("agent") == "masked",
        f"the STACKIT Server Agent is masked ({v.get('agent')})",
    )
    check(v.get("debian_user") == "absent", "no default user without break-glass")
    check(
        v.get("costguard_user") == "/usr/sbin/nologin",
        "costguard is a system user without a shell",
    )
    check(
        v.get("env_file") == "600:root",
        f"the environment file is 0600 root ({v.get('env_file')})",
    )
    check(
        v.get("env_readable_by_costguard") == "no",
        "the costguard user can't read the environment file",
    )
    listeners = v.get("listeners", "")
    check(
        ":22 " not in listeners + " " and "5355" not in listeners,
        f"no SSH or LLMNR listener ({listeners})",
    )
    check(
        v.get("timezone") == "Europe/Berlin",
        f"the server runs in Europe/Berlin ({v.get('timezone')})",
    )
    check(v.get("reboot_config") == "3", "unattended-upgrades reboots at 04:00")
    check(
        sorted(timers)
        == sorted(
            [
                "costguard-budgets.timer/active/costguard@budgets.service/Mon..Fri_*-*-*_10:00:00_Europe/Berlin",
                "costguard-delete.timer/active/costguard@delete.service/Tue_*-*-*_08:00:00_Europe/Berlin",
                "costguard-report.timer/active/costguard@flag.service/Mon_*-*-*_08:00:00_Europe/Berlin",
            ]
        ),
        f"timers: {timers}",
    )
    check(
        number(v.get("exposure_job")) <= 2.0,
        f"systemd-analyze security of a job <= 2.0 ({v.get('exposure_job')})",
    )
    check(
        number(v.get("exposure_install")) <= 4.0,
        f"systemd-analyze security of the install <= 4.0 ({v.get('exposure_install')})",
    )

    if args.scenario == "happy":
        check(
            v.get("install") == "active/success",
            f"the binary was installed ({v.get('install')})",
        )
        check(
            v.get("binary") == "755:root",
            f"/usr/local/bin/costguard is 0755 root ({v.get('binary')})",
        )
        check(
            v.get("version") == args.version,
            f"the binary reports {args.version} ({v.get('version')})",
        )
        check(
            v.get("boot", "").startswith("inactive/")
            or v.get("boot", "").startswith("failed/"),
            f"the boot run ran once ({v.get('boot')})",
        )
        check(
            any(args.version in p for p in posts),
            f"the boot run posted to the chat ({len(posts)} posts)",
        )
        check(
            not any("invalid configuration" in p for p in posts),
            "costguard accepts the config Terraform wrote",
        )
        check(
            number(v.get("lock_wait_seconds"), 0) >= 15,
            f"a run waits for the lock ({v.get('lock_wait_seconds')} s)",
        )
    else:
        check(
            v.get("install") == "failed/exit-code",
            f"the install refused the binary ({v.get('install')})",
        )
        check(v.get("binary") == "missing", "no binary was installed")
        check(
            any("does not match the pinned SHA-256 hash" in p for p in posts),
            f"the failure was posted ({len(posts)} posts)",
        )
        check(
            "costguard-install: install failed" in serial,
            "the failure is on the serial console",
        )
        check(len(posts) == 1, f"exactly one message ({len(posts)})")

    print(
        f"check.py: available memory after boot: {v.get('mem_available_mb')} MB; failed units: {v.get('failed_units') or '-'}"
    )
    for p in posts:
        print("  post: " + p[:300])
    if failures:
        print(f"check.py: {len(failures)} check(s) failed")
        return 1
    print("check.py: all checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
