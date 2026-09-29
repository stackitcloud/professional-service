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

BINARY    := costguard-vm
DIST      := dist
PACKAGE   := ./...
# Linked into the binary; releases set it: make dist VERSION=v0.1.0
VERSION   ?= dev
PLATFORMS := linux/amd64 linux/arm64
# sha256sum on Linux, shasum on macOS; both write the same format.
SHA256    := $(shell command -v sha256sum 2>/dev/null || echo shasum -a 256)

.PHONY: build dist test lint

# Compiles the static single binary for this machine to dist/costguard-vm.
build:
	CGO_ENABLED=0 go build -trimpath -o $(DIST)/$(BINARY) ./cmd/costguard-vm

# Builds the release files into dist/: one static binary per platform,
# named costguard-vm_<version>_linux_<arch>, plus SHA256SUMS. -trimpath and
# -buildvcs=false make the build reproducible: the same Go version builds
# the same bytes anywhere, so a release can be rebuilt and its hashes
# compared.
dist:
	rm -rf $(DIST)
	mkdir -p $(DIST)
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -buildvcs=false \
			-ldflags "-s -w -X main.version=$(VERSION)" \
			-o $(DIST)/$(BINARY)_$(VERSION)_$${os}_$${arch} ./cmd/costguard-vm || exit 1; \
	done
	cd $(DIST) && $(SHA256) $(BINARY)_$(VERSION)_* > SHA256SUMS
	cat $(DIST)/SHA256SUMS

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
