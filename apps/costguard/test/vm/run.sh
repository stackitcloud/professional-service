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

# Boots costguard's real cloud-init in a local QEMU VM (Debian 13 cloud image).
#
#   test/vm/run.sh [happy|mismatch|all]      (default: all)
#
# happy:    the binary installs, the boot run posts, sandbox, timers and hardening are checked.
# mismatch: a wrong pinned hash; the install must refuse the binary and post the failure.
#
# Needs: KVM, qemu-system-x86_64 + qemu-img, OVMF, socat, openssl, python3, terraform, go.
# Settings via environment: QEMU, QEMU_IMG, OVMF_CODE, TERRAFORM, VMTEST_DIR, DEBIAN_IMAGE.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
APP=$(cd "$HERE/../.." && pwd)
WORK=${VMTEST_DIR:-$APP/build/vmtest}
QEMU=${QEMU:-qemu-system-x86_64}
QEMU_IMG=${QEMU_IMG:-qemu-img}
TERRAFORM=${TERRAFORM:-terraform}
IMAGE_BASE=https://cloud.debian.org/images/cloud/trixie/latest
IMAGE_NAME=debian-13-genericcloud-amd64.qcow2
VERSION=vmtest
HOST_IP=169.254.0.2
SEED_PORT=18080
METADATA_PORT=18081
WEBHOOK_PORT=18443
DOWNLOAD_PORT=18444
WEBHOOK_SECRET=vmtest-webhook-secret-5f1c

die() {
	echo "run.sh: $*" >&2
	exit 1
}

find_ovmf() {
	local f
	for f in "${OVMF_CODE:-}" /usr/share/edk2/x64/OVMF_CODE.4m.fd /usr/share/OVMF/OVMF_CODE_4M.fd \
		/usr/share/edk2-ovmf/x64/OVMF_CODE.fd /usr/share/OVMF/OVMF_CODE.fd; do
		if [ -n "$f" ] && [ -f "$f" ]; then
			echo "$f"
			return
		fi
	done
	die "no OVMF firmware found; set OVMF_CODE"
}

prepare() {
	mkdir -p "$WORK/tls" "$WORK/downloads/$VERSION"
	[ -e /dev/kvm ] || die "/dev/kvm is missing"

	IMAGE=${DEBIAN_IMAGE:-$WORK/$IMAGE_NAME}
	if [ ! -f "$IMAGE" ]; then
		echo "run.sh: downloading $IMAGE_NAME"
		curl -fsSL --proto '=https' -o "$WORK/SHA512SUMS" "$IMAGE_BASE/SHA512SUMS"
		curl -fSL --proto '=https' -o "$IMAGE.part" "$IMAGE_BASE/$IMAGE_NAME"
		(cd "$WORK" && grep " $IMAGE_NAME\$" SHA512SUMS | sed "s| $IMAGE_NAME\$| $(basename "$IMAGE").part|" | sha512sum -c -)
		mv "$IMAGE.part" "$IMAGE"
	fi

	make -s -C "$APP" dist VERSION="$VERSION" >/dev/null
	cp "$APP/dist/costguard_${VERSION}_linux_amd64" "$WORK/downloads/$VERSION/"
	SHA=$(sha256sum "$APP/dist/costguard_${VERSION}_linux_amd64" | cut -d' ' -f1)

	if [ ! -f "$WORK/tls/server.pem" ]; then
		openssl req -x509 -newkey rsa:2048 -nodes -days 30 -subj "/CN=costguard vmtest CA" \
			-addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" \
			-keyout "$WORK/tls/ca.key" -out "$WORK/tls/ca.pem" 2>/dev/null
		openssl req -newkey rsa:2048 -nodes -subj "/CN=$HOST_IP" \
			-keyout "$WORK/tls/server.key" -out "$WORK/tls/server.csr" 2>/dev/null
		printf 'subjectAltName=IP:%s\nextendedKeyUsage=serverAuth\nkeyUsage=critical,digitalSignature,keyEncipherment\nsubjectKeyIdentifier=hash\nauthorityKeyIdentifier=keyid,issuer\n' "$HOST_IP" >"$WORK/tls/ext.cnf"
		openssl x509 -req -in "$WORK/tls/server.csr" -CA "$WORK/tls/ca.pem" -CAkey "$WORK/tls/ca.key" \
			-CAcreateserial -days 30 -extfile "$WORK/tls/ext.cnf" -out "$WORK/tls/server.pem" 2>/dev/null
	fi

	"$TERRAFORM" -chdir="$HERE/render" init -backend=false -input=false >/dev/null
}

render() {
	local dir=$1 sha=$2
	mkdir -p "$dir/seed"
	"$TERRAFORM" -chdir="$HERE/render" apply -auto-approve -input=false -state="$dir/render.tfstate" \
		-var "webhook_url=https://$HOST_IP:$WEBHOOK_PORT/hook/$WEBHOOK_SECRET" \
		-var "download_url=https://$HOST_IP:$DOWNLOAD_PORT/{version}/" \
		-var "binary_version=$VERSION" -var "binary_sha256=$sha" \
		-var 'features={ delete = { enabled = true }, budgets = { enabled = true, limits = [{ name = "Org", organization = true, monthly_eur = 100 }] } }' >/dev/null
	"$TERRAFORM" -chdir="$HERE/render" output -state="$dir/render.tfstate" -raw user_data >"$dir/user-data.real"
	python3 "$HERE/compose.py" --real "$dir/user-data.real" --ca "$WORK/tls/ca.pem" \
		--collector "$HERE/collect.sh" --out "$dir/seed/user-data"
	printf 'instance-id: vmtest-%s\nlocal-hostname: costguard\n' "$(date +%s)" >"$dir/seed/meta-data"
	: >"$dir/seed/vendor-data"
	printf 'version: 2\nethernets:\n  nic0:\n    match: {name: "en*"}\n    dhcp4: true\n' >"$dir/seed/network-config"
}

boot() {
	local dir=$1 ovmf
	ovmf=$(find_ovmf)
	rm -f "$dir/disk.qcow2" "$dir/serial.log" "$dir/webhook.jsonl"
	"$QEMU_IMG" create -q -f qcow2 -F qcow2 -b "$IMAGE" "$dir/disk.qcow2" 10G

	python3 "$HERE/fake_services.py" --dir "$dir" --metadata-port $METADATA_PORT --seed-port $SEED_PORT \
		--download-port $DOWNLOAD_PORT --webhook-port $WEBHOOK_PORT &
	SERVICES=$!
	ln -sfn "$WORK/tls" "$dir/tls"
	ln -sfn "$WORK/downloads" "$dir/downloads"
	sleep 1

	echo "run.sh: booting $(basename "$dir") (serial log: $dir/serial.log)"
	timeout 20m "$QEMU" -machine q35,accel=kvm -cpu host -smp 1 -m 1024 \
		-drive if=pflash,format=raw,readonly=on,file="$ovmf" \
		-drive file="$dir/disk.qcow2",if=virtio,format=qcow2 \
		-smbios "type=1,serial=ds=nocloud;s=http://$HOST_IP:$SEED_PORT/" \
		-netdev "user,id=n0,net=169.254.0.0/16,host=$HOST_IP,dns=169.254.0.3,dhcpstart=169.254.0.15,guestfwd=tcp:169.254.169.254:80-cmd:socat - TCP:127.0.0.1:$METADATA_PORT" \
		-device virtio-net-pci,netdev=n0 \
		-display none -monitor none -serial file:"$dir/serial.log" || echo "run.sh: QEMU ended with $?"
	kill "$SERVICES" 2>/dev/null || true
	wait "$SERVICES" 2>/dev/null || true
}

run_scenario() {
	local name=$1 dir="$WORK/$1" sha
	case $name in
	happy) sha=$SHA ;;
	mismatch) sha=$(printf '%s' "not the binary" | sha256sum | cut -d' ' -f1) ;;
	*) die "unknown scenario $name" ;;
	esac
	render "$dir" "$sha"
	boot "$dir"
	python3 "$HERE/check.py" --scenario "$name" --serial "$dir/serial.log" --webhook "$dir/webhook.jsonl" \
		--secret "$WEBHOOK_SECRET" --version "$VERSION"
}

main() {
	local scenarios=("${@:-all}") failed=0 s
	[ "${scenarios[0]}" = all ] && scenarios=(happy mismatch)
	prepare
	for s in "${scenarios[@]}"; do
		run_scenario "$s" || failed=1
	done
	return $failed
}

main "$@"
