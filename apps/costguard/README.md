<!-- tags: cost, cleanup, automation, iaas, terraform -->

# costguard

**costguard keeps your STACKIT cloud tidy and installs with one `terraform apply`.** It looks for things that cost
money but are not used by anything, reports them in a chat channel and, once you switch deletion on, deletes them.
It runs on a small server that logs in with the service account attached to it, so no key is stored on the server.

> **Work in progress:** the full README, including the checks and the labels, follows. Install from a release tag
> (`apps/costguard/vX.Y.Z`): its Terraform code in [`terraform/`](terraform/) (settings:
> [`terraform.tfvars.example`](terraform/terraform.tfvars.example)) pins the binary the server downloads.

## Developing

- `make test`: unit tests with the race detector and an 80 % coverage gate per package.
- `make lint`: `go vet` for the release build and the dev build.
- `make dist VERSION=v0.1.0`: the release files in `dist/`, one static binary each for linux/amd64 and linux/arm64
  plus `SHA256SUMS`. The build is reproducible: it uses the official Go toolchain of `go.mod`'s version (Go
  downloads it if yours differs; distro builds of the same version build other bytes), so every machine builds the
  same bytes.
- `make tf-test`: `terraform fmt`, `validate` and `terraform test` (the STACKIT provider is mocked, no cloud access)
  plus the install script's unit tests.
- `make vmtest`: boots the real cloud-init in a local QEMU VM (Debian 13 cloud image, 1 vCPU / 1 GB) with the
  `make dist` binary and fakes for the metadata service, the download server and the webhook; checks the install,
  the boot run, the systemd sandbox, the timers and the hardening. Needs KVM, QEMU, OVMF, socat and openssl; see
  [`test/vm/run.sh`](test/vm/run.sh). Run it after every change to the cloud-init module.
- Release binaries log in only with the service account attached to the server. To run costguard on your own
  machine with a service account key, use the dev build tag:

  ```bash
  export STACKIT_SERVICE_ACCOUNT_KEY_PATH=~/path/to/key.json
  export COSTGUARD_WEBHOOK_URL='https://...'
  go run -tags dev ./cmd/costguard --config config.yaml report
  ```

## Releasing

1. `make pin VERSION=vX.Y.Z` builds the release files and writes their hashes into
   [`terraform/025-release.tf`](terraform/025-release.tf) (it refuses a version that is tagged already).
2. Commit that file and open a pull request; CI (`make pin-check`) rebuilds and fails if the pin doesn't match.
3. The push to `main` runs [`costguard-release`](../../.github/workflows/costguard-release.yaml): tests, a rebuild
   compared with the pin, then the Forgejo release of tag `apps/costguard/vX.Y.Z` with both binaries and
   `SHA256SUMS`, and an anonymous download check through the pinned URL.

`main` keeps the latest release's pin until the next one, so a checkout of `main` installs the last release's binary
with possibly newer Terraform code: install from a tag. Test builds set `binary_override` and `download_url`
(see [`terraform.tfvars.example`](terraform/terraform.tfvars.example)).
