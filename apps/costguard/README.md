<!-- tags: cost, cleanup, budgets, automation, iaas, terraform -->

# costguard

**costguard keeps your STACKIT cloud tidy and installs with one `terraform apply`.** It looks for things that cost
money but are not used by anything, reports them in a chat channel and, once you switch deletion on, deletes them.
It also watches monthly budgets and posts when one passes a threshold. It runs on a small server that logs in with
the service account attached to it, so no key is stored on the server.

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
- `make tf-test`: `terraform fmt`, `validate` and `terraform test` (the STACKIT and http providers are mocked, no
  cloud access or download) plus the install script's unit tests.
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

1. Set the new `version` in [`terraform/000-release.tf`](terraform/000-release.tf) (`vMAJOR.MINOR.PATCH`, optionally
   `-pre.release`), commit it and open a pull request.
2. The push to `main` runs [`costguard-release`](../../.github/workflows/costguard-release.yaml): tests, the release
   build (`make dist`), then the Forgejo release of tag `apps/costguard/vX.Y.Z` on that commit with both binaries and
   `SHA256SUMS`, and an anonymous download check of all three through the pinned URL. Nothing happens for a version
   that is released already.
3. `terraform plan` reads the hashes from that `SHA256SUMS` (so it needs the release to exist and Forgejo to be
   reachable) and shows them in the `binary` output; the server installs only a binary with that hash. The build is
   reproducible: `make dist VERSION=vX.Y.Z` at the tag builds the same files, if you want to compare.

`main` keeps the latest release's version until the next one, so a checkout of `main` installs the last release's
binary with possibly newer Terraform code: install from a tag. Test builds set `binary_override` (with their own
hashes; nothing is read at plan time) and `download_url` (see
[`terraform.tfvars.example`](terraform/terraform.tfvars.example)).
