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

# Changelog

All notable changes to costguard are documented in this file.

## [0.0.1] — initial pre-release (v1)

### Added

- costguard v1: STACKIT cloud cost-hygiene bot with one image and three
  subcommands — `scan`, `delete`, `callback`.
- Scanners: stale projects and empty SNAs (report-only hygiene section), idle public IPs and detached volumes (deletion candidates with
  mark labels), 30-day billing with top-10 projects and 7-day cost
  anomaly detection.
- Deletion runner: due-candidates only (expired mark labels), IPs before
  volumes, re-validation immediately before every DELETE, retry with
  exponential backoff on 429/5xx/409, 404-on-delete treated as success,
  snapshot pre-deletion for volumes, 1 s pacing between deletions.
- Protection model: safe label (portal-native), mark label
  (`costguard-delete-after`, compact-UTC value), and a merged whitelist
  from env CSV + YAML + a dedicated STACKIT Secrets Manager KV v2 entry.
- Outputs: Google Chat Cards V2, Slack Block Kit and Teams Adaptive Cards,
  all with collapsible sections, estimated monthly savings, top-10
  projects, anomaly alert, 30-day cost chart (S3 presigned URL) and
  signed "Do not delete" buttons when the callback is configured.
- Callback server: `GET`/`POST /protect` with HMAC + expiry validation,
  410 Gone for expired buttons and deleted resources, idempotent KV v2
  compare-and-swap whitelist append, `/healthz`.
- Two-stage rollout manifests: stage 1 weekly scan CronJob only; stage 2
  adds the weekly delete CronJob (1 h after scan) and the callback
  Deployment, each with its own least-privilege service account.
- Build & quality: Makefile (build/test/lint) with a per-package
  80% coverage gate, go vet, ko-built multi-arch image
  (`cgr.dev/chainguard/static`, non-root, OCI labels, SPDX SBOM), repo CI
  (test + lint, path-filtered to `apps/costguard/**`) and tag-triggered
  image publication via ko.
