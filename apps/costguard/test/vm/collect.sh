#!/bin/bash
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


# Runs inside the QEMU test VM (added by run.sh as a second cloud-config
# part, never part of the real user data). Waits until the costguard jobs
# have settled, prints "VMTEST key=value" lines to the serial console, then
# powers the VM off. check.py evaluates them.
set -u

# agetty hangs up ttyS0 when it starts (vhangup), which kills descriptors
# opened before; so wait for it, and open the port for every write.
for _ in $(seq 1 30); do
	systemctl is-active --quiet serial-getty@ttyS0.service && break
	sleep 2
done
say() { printf 'VMTEST %s\n' "$*" >/dev/ttyS0; }
log() { sed 's/^/VMTEST-LOG /' >/dev/ttyS0; }

# The boot run waits for the install, which may retry; give both time.
for _ in $(seq 1 180); do
	busy=$(systemctl list-jobs --no-legend 2>/dev/null | grep -c costguard)
	state=$(systemctl show -p ActiveState --value costguard-install.service costguard@boot.service | tr '\n' ' ')
	if [ "$busy" -eq 0 ] && ! grep -q activating <<<"$state"; then
		break
	fi
	sleep 5
done

say "cloud_init=$(cloud-init status 2>/dev/null | awk '{print $2}')"
say "install=$(systemctl show -p ActiveState --value costguard-install.service)/$(systemctl show -p Result --value costguard-install.service)"
say "boot=$(systemctl show -p ActiveState --value costguard@boot.service)/$(systemctl show -p Result --value costguard@boot.service)/$(systemctl show -p ExecMainStatus --value costguard@boot.service)"
say "binary=$(stat -c '%a:%U' /usr/local/bin/costguard 2>/dev/null || echo missing)"
say "version=$(/usr/local/bin/costguard --version 2>/dev/null || echo none)"
say "env_file=$(stat -c '%a:%U' /etc/costguard/costguard.env)"
say "env_readable_by_costguard=$(runuser -u costguard -- cat /etc/costguard/costguard.env >/dev/null 2>&1 && echo yes || echo no)"
say "ssh=$(systemctl is-enabled ssh.service 2>&1)/$(systemctl is-enabled ssh.socket 2>&1)"
say "ssh_unix=$(systemctl is-enabled sshd-unix-local.socket 2>&1)/$(systemctl is-active sshd-unix-local.socket 2>&1)/$(ss -Hxl | grep -c ssh-unix-local)"
say "agent=$(systemctl is-enabled stackit-server-agent.service 2>&1)"
say "debian_user=$(getent passwd debian >/dev/null && echo present || echo absent)"
say "costguard_user=$(getent passwd costguard | cut -d: -f7)"
say "listeners=$(ss -H -tuln | awk '{print $1 "/" $5}' | sort | tr '\n' ' ')"
say "timezone=$(timedatectl show -p Timezone --value)"
say "reboot_config=$(apt-config dump | grep -cE '^Unattended-Upgrade::Automatic-Reboot(-Time|-WithUsers)? ')"
for t in $(systemctl list-unit-files --no-legend 'costguard-*.timer' | awk '{print $1}'); do
	say "timer=$t/$(systemctl is-active "$t")/$(systemctl show -p Unit --value "$t")/$(systemctl show -p TimersCalendar --value "$t" | sed -e 's/.*OnCalendar=\([^;]*\).*/\1/' -e 's/ *$//' | tr ' ' '_')"
done
exposure() {
	systemd-analyze security --no-pager "$1" 2>/dev/null | grep -o 'Overall exposure level for [^:]*: [0-9.]*' | awk '{print $NF}'
}
say "exposure_job=$(exposure costguard@report.service)"
say "exposure_install=$(exposure costguard-install.service)"
say "mem_available_mb=$(free -m | awk '/^Mem:/ {print $7}')"
say "failed_units=$(systemctl --failed --no-legend --plain | awk '{print $1}' | tr '\n' ' ')"

# One run at a time: while another process holds the lock, a run waits.
if [ -x /usr/local/bin/costguard ]; then
	systemd-run --quiet --unit=vmtest-lock-holder -p User=costguard -p Group=costguard \
		-p RuntimeDirectory=costguard -p RuntimeDirectoryPreserve=yes /usr/bin/flock /run/costguard/lock sleep 20
	sleep 2
	start=$(date +%s)
	systemctl start costguard@flag.service
	say "lock_wait_seconds=$(($(date +%s) - start))"
	say "flag=$(systemctl show -p Result --value costguard@flag.service)"
fi

journalctl -b -u costguard-install.service -o cat --no-pager | tail -20 | log
journalctl -b -u costguard@boot.service -o cat --no-pager | tail -20 | log
say DONE
sync
systemctl poweroff
