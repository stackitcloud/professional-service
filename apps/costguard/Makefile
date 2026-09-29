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

BINARY    := costguard
DIST      := dist
PACKAGE   := ./...
# Linked into the binary; releases set it: make dist VERSION=v0.1.0
VERSION   ?= dev
PLATFORMS := linux/amd64 linux/arm64
# sha256sum on Linux, shasum on macOS; both write the same format.
SHA256    := $(shell command -v sha256sum 2>/dev/null || echo shasum -a 256)
TERRAFORM ?= terraform
# Release files are built with the official toolchain of go.mod's version:
# distro builds of the same version (e.g. with a GOEXPERIMENT) build other
# bytes. Go downloads it (checked against sum.golang.org) if the local one
# differs.
RELEASE_TOOLCHAIN := go$(shell sed -n 's/^go //p' go.mod)
TAG_PREFIX := apps/costguard/

.PHONY: build dist pin pin-check test lint tf-test vmtest

# Compiles the static single binary for this machine to dist/costguard.
build:
	CGO_ENABLED=0 go build -trimpath -o $(DIST)/$(BINARY) ./cmd/costguard

# Builds the release files into dist/: one static binary per platform,
# named costguard_<version>_linux_<arch>, plus SHA256SUMS. -trimpath,
# -buildvcs=false and the official toolchain make the build reproducible:
# it builds the same bytes on any machine, so a release can be rebuilt and
# its hashes compared.
dist:
	rm -rf $(DIST)
	mkdir -p $(DIST)
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		GOTOOLCHAIN=$(RELEASE_TOOLCHAIN) CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -buildvcs=false \
			-ldflags "-s -w -X main.version=$(VERSION)" \
			-o $(DIST)/$(BINARY)_$(VERSION)_$${os}_$${arch} ./cmd/costguard || exit 1; \
	done
	cd $(DIST) && $(SHA256) $(BINARY)_$(VERSION)_* > SHA256SUMS
	cat $(DIST)/SHA256SUMS

# Pins a release: builds the release files and writes their hashes into
# terraform/025-release.tf. Commit that file; its push releases the version
# (.github/workflows/costguard-release.yaml). Refuses a version whose tag
# exists.
pin:
	@case "$(VERSION)" in v[0-9]*) ;; *) echo "usage: make pin VERSION=vX.Y.Z" >&2; exit 2 ;; esac
	@git ls-remote --exit-code --tags origin "refs/tags/$(TAG_PREFIX)$(VERSION)" >/dev/null 2>&1; rc=$$?; \
	if [ $$rc -eq 0 ]; then echo "$(TAG_PREFIX)$(VERSION) exists already: pin the next version" >&2; exit 1; fi; \
	if [ $$rc -ne 2 ]; then echo "cannot list the tags of origin (git ls-remote exit $$rc)" >&2; exit 1; fi
	$(MAKE) --no-print-directory dist VERSION=$(VERSION)
	python3 scripts/pin.py write --version $(VERSION) --sums $(DIST)/SHA256SUMS

# Checks an unreleased pin: rebuilds the pinned version and compares the
# hashes with terraform/025-release.tf. Nothing to check when nothing is
# pinned or the pinned version is released (its tag exists).
pin-check:
	@v="$$(python3 scripts/pin.py version)" || exit 1; \
	if [ -z "$$v" ]; then echo "pin-check: no release pinned"; exit 0; fi; \
	git ls-remote --exit-code --tags origin "refs/tags/$(TAG_PREFIX)$$v" >/dev/null 2>&1; rc=$$?; \
	if [ $$rc -eq 0 ]; then echo "pin-check: $$v is released (its tag exists), nothing to check"; exit 0; fi; \
	if [ $$rc -ne 2 ]; then echo "pin-check: cannot list the tags of origin (git ls-remote exit $$rc)" >&2; exit 1; fi; \
	$(MAKE) --no-print-directory dist VERSION="$$v" && python3 scripts/pin.py check --sums $(DIST)/SHA256SUMS

# Runs all unit tests with -race and generates coverage.out,
# then enforces the per-package 80% coverage gate. Exempt: cmd/ (a thin
# flag-parsing stub; the logic lives in internal/app) and internal/fake
# (the in-memory STACKIT used by the tests). The login tests also run
# with the dev build tag; the gate counts the release build only.
test:
	go test -race -tags dev ./internal/stackit/
	go test -race -coverprofile=coverage.out $(PACKAGE)
	@awk ' \
		{ \
			if ($$1 !~ /\.go:/) next; \
			file = $$1; sub(/:.*/, "", file); \
			n = split(file, parts, "/"); \
			dir = parts[1]; \
			for (i = 2; i < n; i++) dir = dir "/" parts[i]; \
			if (dir ~ /\/cmd\// || dir ~ /\/internal\/fake$$/) next; \
			total[dir] += $$2; \
			if ($$3 + 0 > 0) covered[dir] += $$2; \
		} \
		END { \
			fail = 0; \
			for (dir in total) { \
				pct = (total[dir] > 0) ? 100 * covered[dir] / total[dir] : 100; \
				printf "coverage %-70s %5.1f%% (%d/%d statements)\n", dir, pct, covered[dir], total[dir]; \
				if (pct < 80) { printf "coverage gate failed: %s below 80%%\n", dir > "/dev/stderr"; fail = 1; } \
			} \
			exit fail; \
		} \
	' coverage.out

# Runs go vet, the standard Go correctness checks (also run by `go test`),
# for the release build and the dev build.
lint:
	go vet $(PACKAGE)
	go vet -tags dev $(PACKAGE)

# Checks the Terraform code offline: fmt, validate, the Terraform tests
# (STACKIT provider mocked), the install script's and the pin script's unit
# tests.
tf-test:
	# Not "." : a local terraform.tfvars is nobody else's business.
	cd terraform && $(TERRAFORM) fmt -check -recursive $$(ls *.tf) modules tests ../test
	$(TERRAFORM) -chdir=terraform init -backend=false -input=false >/dev/null
	$(TERRAFORM) -chdir=terraform validate
	$(TERRAFORM) -chdir=terraform test
	python3 -m unittest discover -s terraform/modules/cloud-init/files -p 'test_*.py'
	python3 -m unittest discover -s scripts -p 'test_*.py'

# Boots the real cloud-init in a local QEMU VM with the make dist binary
# and fake STACKIT services (needs KVM); see test/vm/run.sh.
vmtest:
	TERRAFORM=$(TERRAFORM) test/vm/run.sh
