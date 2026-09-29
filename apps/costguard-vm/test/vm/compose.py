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


"""Builds the QEMU test's user data: the real cloud-config (unchanged) plus a
second part with only what the test needs, merged by cloud-init: the test
CA (the fake webhook and download server use HTTPS) and the collector.
"""

import argparse
import json
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--real", required=True)
    ap.add_argument("--ca", required=True)
    ap.add_argument("--collector", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    with open(args.real, encoding="utf-8") as f:
        real = f.read()
    with open(args.ca, encoding="utf-8") as f:
        ca = f.read()
    with open(args.collector, encoding="utf-8") as f:
        collector = f.read()

    test = {
        # Append lists (write_files, runcmd) instead of replacing them.
        "merge_how": [
            {"name": "list", "settings": ["append"]},
            {"name": "dict", "settings": ["no_replace", "recurse_list"]},
        ],
        "ca_certs": {"trusted": [ca]},
        "write_files": [
            {
                "path": "/usr/local/sbin/costguard-vmtest-collect",
                "permissions": "0755",
                "content": collector,
            }
        ],
        # After cloud-final, so `cloud-init status` is final when it reports.
        "runcmd": [
            [
                "systemd-run",
                "--no-block",
                "--unit=costguard-vmtest-collect",
                "-p",
                "After=cloud-final.service",
                "/usr/local/sbin/costguard-vmtest-collect",
            ]
        ],
    }

    msg = MIMEMultipart()
    msg.attach(MIMEText(real, "cloud-config"))
    # JSON is YAML.
    msg.attach(
        MIMEText("#cloud-config\n" + json.dumps(test, indent=1) + "\n", "cloud-config")
    )
    with open(args.out, "w", encoding="utf-8") as f:
        f.write(msg.as_string())


if __name__ == "__main__":
    main()
