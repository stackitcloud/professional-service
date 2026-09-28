<!--
  Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
-->

# Changelog

All notable changes to costguard are documented in this file. From v0.1.0
on, every merge is a release; the GitHub release notes list each change.

## [0.1.0] — unreleased

A rewrite into a lightweight, self-hosted cleanup tool with two stages.

### Changed

- Three subcommands for two stages: `report` (stage 1, read-only), `flag`
  (stage 2 Monday: post, then label new candidates `delete=true`) and
  `delete` (stage 2 Tuesday).
- Two user-facing labels: `delete=true` (delete in the next delete run) and
  `do-not-delete=true` (never touch; on a folder or project: skip it).
- Configuration is one strict YAML file (unknown keys are errors) plus the
  webhook URL from the environment. Scope and skip lists take IDs or names.
- Messages: the saving of each run (Monday: what the listed items would save; Tuesday: what the run's deletions save, per month and per year) and per category, links to the project in the portal,
  Monday always posts, Tuesday only when something happened.
- Image: ko on `gcr.io/distroless/static:nonroot` (amd64 + arm64), released
  to ghcr on every merge with digest-pinned manifests.

### Added

- Delete on request: servers, volumes, public IPs, snapshots, images,
  network interfaces and security groups labelled `delete=true`.
- Warnings for empty projects (older than 30 days, €0 spend, no resources)
  and empty network areas.
- Public IPs used by network or application load balancers, detached
  volumes in SKE projects and volumes with snapshots are never flagged.
- A skip entry that matches nothing blocks all flagging and deleting.
- Nothing is deleted that was not in a message: everything marked
  `delete=true` is always listed; new finds are capped at 10 per category per
  run and only the listed ones are flagged. The rest follows in later runs.
- A broken configuration is posted to the chat (with every problem listed)
  as long as `output` and the webhook are usable; a second YAML document in
  the config file is refused instead of ignored.
- The delete run retries its re-reads on rate limiting and server errors,
  and a disk or IP whose server cannot be checked keeps its label.
- Chat messages are retried up to three times on rate limiting (429),
  server errors (5xx) and network errors.
- `warnEmptyAfterDays: 0` switches the empty-project and empty-network-area
  warnings off (for installs without organization-wide cost access).
- Blocked-run messages say that skip entries must point inside the scope.
- Scan errors are grouped by cause ("listing images: HTTP 403: … in 300
  places, e.g. …") and listed in full at the end of the message.
- Login problems and an organization (or scope folder) that cannot be read
  fail the run with a clear message instead of a report that says "Nothing
  found".
- If label writes fail after the Monday message, a correction lists what
  will not be deleted. Errors in messages read "HTTP 403: <API message>"
  instead of raw SDK text; stopped runs say why in plain words.
- Messages are sent with their own time budget, so an interrupted run still
  reports what it did; interrupted delete runs say so.
- A disk or IP that carries both `do-not-delete` and `delete=true` gets the
  stale `delete` label removed by the Monday run; other kinds with both labels
  are listed for manual cleanup.
- A disk or IP attached to a server marked `delete=true` keeps its label
  when the server's delete fails, and goes together with it in the next run.
- Only `eu01` is scanned unless `regions` lists more.
- Webhook URLs must use `https`.

### Fixed

- The org walk followed only the first 50 projects or folders per
  container; it now follows pagination.
- The Google Chat and Slack payloads did not match the Cards v2 and Block
  Kit formats.

### Removed

- The callback server and buttons, the Secrets Manager whitelist, the S3
  cost chart, top-10 and anomaly sections, the grace period and dry-run
  switch, the per-resource whitelists and the Dockerfile. Dependencies on
  the AWS SDK, the Vault client and the chart library are gone.

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
