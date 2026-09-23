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

BINARY  := costguard
DIST    := dist
PACKAGE := ./...

.PHONY: build test lint docker coverage

# Compiles the static single binary to dist/costguard.
build:
	CGO_ENABLED=0 go build -o $(DIST)/$(BINARY) ./cmd/costguard

# Runs all unit tests with -race and generates coverage.out,
# then enforces the per-package 80% coverage gate. The cmd/ entrypoint
# package is exempt: it is a ~20-line dispatch stub with no business logic
# all testable logic lives in internal/app.
test:
	go test -race -coverprofile=coverage.out $(PACKAGE)
	@awk ' \
		{ \
			if ($$1 !~ /\.go:/) next; \
			file = $$1; sub(/:.*/, "", file); \
			n = split(file, parts, "/"); \
			dir = parts[1]; \
			for (i = 2; i < n; i++) dir = dir "/" parts[i]; \
			if (dir ~ /\/cmd\//) next; \
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

# Runs go vet, the standard Go correctness checks (also run by `go test`).
lint:
	go vet $(PACKAGE)

# Builds the Docker image.
docker:
	docker build -t costguard:dev .
