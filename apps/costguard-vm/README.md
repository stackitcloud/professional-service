<!-- tags: cost, cleanup, automation, iaas, terraform -->

# costguard-vm

**costguard-vm keeps your STACKIT cloud tidy and installs with one `terraform apply`.** It is the VM edition of
costguard: it looks for things that cost money but are not used by anything, reports them in a chat
channel and, once you switch deletion on, deletes them. It runs on a small server that logs in with the service
account attached to it, so no key is stored on the server.

> **Work in progress:** costguard-vm is not released yet. The Terraform code is in [`terraform/`](terraform/)
> (settings: [`terraform.tfvars.example`](terraform/terraform.tfvars.example)); the release and the full README
> follow. Until then, [costguard's README](../costguard/README.md) explains the checks and the labels.

## Developing

- `make test`: unit tests with the race detector and an 80 % coverage gate per package.
- `make lint`: `go vet` for the release build and the dev build.
- `make dist VERSION=v0.1.0`: the release files in `dist/`, one static binary each for linux/amd64 and linux/arm64
  plus `SHA256SUMS`. The build is reproducible: the same Go version builds the same bytes.
- `make tf-test`: `terraform fmt`, `validate` and `terraform test` (the STACKIT provider is mocked, no cloud access)
  plus the install script's unit tests.
- `make vmtest`: boots the real cloud-init in a local QEMU VM (Debian 13 cloud image, 1 vCPU / 1 GB) with the
  `make dist` binary and fakes for the metadata service, the download server and the webhook; checks the install,
  the boot run, the systemd sandbox, the timers and the hardening. Needs KVM, QEMU, OVMF, socat and openssl; see
  [`test/vm/run.sh`](test/vm/run.sh). Run it after every change to the cloud-init module.
- Release binaries log in only with the service account attached to the server. To run costguard-vm on your own
  machine with a service account key, use the dev build tag:

  ```bash
  export STACKIT_SERVICE_ACCOUNT_KEY_PATH=~/path/to/key.json
  export COSTGUARD_WEBHOOK_URL='https://...'
  go run -tags dev ./cmd/costguard-vm --config config.yaml report
  ```
