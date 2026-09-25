<!-- tags: cost, monitoring, automation, iaas, cleanup -->

# costguard — STACKIT cost-hygiene bot

## 1. What it does

costguard scans a STACKIT organisation (or a set of folders) and reports the
resources that quietly burn money: **stale projects** (older than
`COSTGUARD_MAX_AGE_DAYS`), **empty network areas** (older than
`COSTGUARD_SNA_MAX_AGE_DAYS`), **detached block-storage volumes**, and **idle
public IPs** (allocated but not attached to any NIC). On top of the resource
scan it pulls 30 days of billing data from the STACKIT Cost API, renders a
cost trend chart (uploaded to S3-compatible object storage when configured),
flags cost anomalies (a jump in the 7-day average spend), and shows the
top-spending projects.

The lifecycle is a **two-phase warning/deletion cycle** driven entirely by a
mark label the bot writes on its deletion candidates (detached volumes and
idle public IPs — the deletion surface equals the cost surface):

1. **Warning phase** (`costguard scan`, weekly): the full scan runs, every new
   candidate is stamped with the label `costguard-delete-after=<deadline>`
   (deadline = now + `COSTGUARD_GRACE_PERIOD`, default 8 h), and a report with
   interactive *"Do not delete"* buttons is sent to Google Chat, Slack, or
   Microsoft Teams.
2. **Execution phase** (`costguard delete`, weekly, 1 h after the scan): the
   bot re-scans, re-validates every candidate (still detached/idle, not
   protected, mark present and **expired**), deletes in the order public IPs →
   volumes (a volume's snapshots go first), and sends a separate confirmation
   with the per-resource result.

Stale projects and empty network areas are **report-only**: an empty
container costs ~€0, so auto-deleting it is not justified by savings.
Deletion is **fail-closed** — a candidate without a valid, expired mark is
never touched, and `COSTGUARD_DRY_RUN=true` (the default) is a global kill
switch: no `DELETE` call is ever issued while it is on.

## 2. Prerequisites

- A **STACKIT service account** per workload (least privilege, one image /
  three subcommands / three accounts):
  - **scan** — Resource Manager *reader*, IaaS *reader + label writer*,
    Cost *reader* (SKE / Object Storage reader for the content probes)
  - **delete** — Resource Manager *reader*, IaaS *reader + deleter +
    label writer* (deletes public IPs, volumes, snapshots), Cost *reader*
  - **callback** — Resource Manager *reader*, IaaS *reader* (existence
    checks) + **write access to the dedicated whitelist Secrets Manager
    instance only** — no deletion rights at all
- STACKIT authentication is handled by the official SDK: a service account
  key file (`STACKIT_SERVICE_ACCOUNT_KEY_PATH`) or a token
  (`STACKIT_SERVICE_ACCOUNT_TOKEN`); on SKE, workload identity is the
  preferred baseline (no long-lived key material).
- A chat **webhook**: a Google Chat space message bot, a Slack incoming
  webhook, or a Teams incoming webhook, depending on
  `COSTGUARD_OUTPUT`.
- For the interactive buttons (stage 2): a **dedicated STACKIT Secrets
  Manager instance** hosting the shared whitelist (KV v2) with a dedicated
  user, and a **publicly reachable URL** for the callback server (e.g. via
  STACKIT ALB).
- Optional, for the cost chart: an S3-compatible **object storage bucket**
  (STACKIT Object Storage works) with an access key/secret pair.

## 3. Configuration reference

All configuration is runtime-only; the binary contains no STACKIT-specific
values. Environment variables take precedence over the YAML config file
(`--config` / `COSTGUARD_CONFIG`).

### STACKIT authentication (read by the SDK, not by costguard)

| Variable | Required | Description |
|---|---|---|
| `STACKIT_SERVICE_ACCOUNT_KEY_PATH` | one of the two | Path to the service account key file (JSON). |
| `STACKIT_SERVICE_ACCOUNT_TOKEN` | one of the two | Short-lived service account token. |
| `STACKIT_REGION` | optional | Region for SDK endpoints when not derivable. |

### Scope (all required)

| Variable | Required | Default | Description |
|---|---|---|---|
| `COSTGUARD_SCOPE` | yes | — | `organisation` or `folder`. |
| `COSTGUARD_ORG_ID` | yes | — | STACKIT organisation UUID. |
| `COSTGUARD_FOLDER_IDS` | if `scope=folder` | — | Comma-separated folder UUIDs. |
| `COSTGUARD_REGIONS` | yes | — | Comma-separated regions to scan (e.g. `eu01`). |
| `COSTGUARD_MAX_AGE_DAYS` | yes | — | Projects older than this (days) are hygiene candidates (≥ 7). |
| `COSTGUARD_SNA_MAX_AGE_DAYS` | yes | — | Network areas older than this (days) are candidates (≥ 7). |
| `COSTGUARD_SAFE_LABEL_KEY` | yes | — | Label key that protects a resource. |
| `COSTGUARD_SAFE_LABEL_VALUE` | yes | — | Label value that protects a resource. |

### Output (required)

| Variable | Required | Default | Description |
|---|---|---|---|
| `COSTGUARD_OUTPUT` | yes | — | `googlechat`, `slack` or `teams` (`prometheus` is rejected at startup). |
| `COSTGUARD_WEBHOOK_URL` | yes (webhook outputs) | — | Webhook URL of the target space/channel. |

### Behavior

| Variable | Required | Default | Description |
|---|---|---|---|
| `COSTGUARD_DRY_RUN` | no | `true` | Global kill switch: no `DELETE` calls when true. |
| `COSTGUARD_GRACE_PERIOD` | no | `8h` | Warning window before a marked candidate becomes deletable (≥ 2h; must exceed the 1 h scan→delete offset). |
| `COSTGUARD_COST_ANOMALY_THRESHOLD_PCT` | no | `20` | Percent increase of the last 7-day spend average over the preceding 7-day average that flags an anomaly. |
| `COSTGUARD_VOLUME_COST_EUR_PER_GB` | no | `0.0619` | Monthly price per GB for the estimated volume savings. |
| `COSTGUARD_PUBLIC_IP_COST_EUR_PER_MONTH` | no | `4.82` | Monthly reservation price per idle public IP for the estimated savings. |

### Interactive buttons + callback (optional group)

| Variable | Required | Default | Description |
|---|---|---|---|
| `COSTGUARD_CALLBACK_URL` | group | — | Public URL of the callback server. When set, notifications carry buttons. |
| `COSTGUARD_CALLBACK_PORT` | if URL set | `8080` | Port the `callback` subcommand listens on. |
| `COSTGUARD_CALLBACK_SECRET` | if URL set | — | HMAC secret for button URL signing (secret store, never literal). |
| `COSTGUARD_WHITELIST_SECRET_PATH` | if URL set | — | KV v2 path of the shared whitelist, `<mount>/<path>` (e.g. `secret/costguard/whitelist`). |
| `COSTGUARD_SM_URL` | if URL set | — | Base URL of the dedicated Secrets Manager instance. |
| `COSTGUARD_SM_USERNAME` | if URL set | — | Dedicated SM user (userpass baseline). |
| `COSTGUARD_SM_PASSWORD` | if URL set | — | Its password. |

When `COSTGUARD_WHITELIST_SECRET_PATH` is set (even without buttons), the
scan and delete runs also **read** the shared whitelist, so protections
granted via a button apply to the next run.

### Cost chart upload (optional group: all five or none)

| Variable | Required | Description |
|---|---|---|
| `COSTGUARD_S3_ENDPOINT` | group | S3-compatible endpoint (e.g. `https://object.storage.eu01.onstackit.cloud`). |
| `COSTGUARD_S3_REGION` | group | Region. |
| `COSTGUARD_S3_ACCESS_KEY` | group | Access key. |
| `COSTGUARD_S3_SECRET_KEY` | group | Secret key. |
| `COSTGUARD_S3_BUCKET` | group | Bucket for the chart objects (7-day presigned links). |

### Whitelist (optional; merged from every source)

A resource listed in **any** source is protected: the env CSVs below, the
`whitelist:` YAML section, and the Secrets Manager entry.

| Variable | Description |
|---|---|
| `COSTGUARD_WHITELIST_PROJECTS` | Project IDs excluded from all candidates and invisible in reports. |
| `COSTGUARD_WHITELIST_FOLDERS` | Folder IDs whose whole subtree is excluded. |
| `COSTGUARD_WHITELIST_VOLUMES` | Volume IDs that are never deleted. |
| `COSTGUARD_WHITELIST_PUBLIC_IPS` | Public IP IDs that are never deleted. |
| `COSTGUARD_WHITELIST_SKIP_VOLUME_SCAN_PROJECTS` | Skip volume scanning for these projects entirely. |
| `COSTGUARD_WHITELIST_SKIP_PUBLIC_IP_SCAN_PROJECTS` | Skip public-IP scanning for these projects entirely. |

### Runtime

| Variable | Required | Default | Description |
|---|---|---|---|
| `COSTGUARD_CONFIG` | no | — | Path to the YAML config file (env vars still win). |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn` or `error`. |

The configuration is validated **before any API call**; a single error lists
every missing or invalid field, and the process exits non-zero.

## 4. Config file example

`config.example.yaml` is a complete, annotated example covering every option.
A minimal organisation-wide dry run:

```yaml
scope: organisation
orgId: 00000000-0000-0000-0000-000000000000
regions: [eu01]
maxAgeDays: 90
snaMaxAgeDays: 60
safeLabelKey: do-not-delete
safeLabelValue: "true"
output: googlechat
webhookUrl: https://chat.googleapis.com/v1/spaces/SPACE_ID/messages?key=KEY&token=TOKEN
dryRun: true
gracePeriod: 8h
```

## 5. Running with Docker

Build the image with [ko](https://ko.build/) (static binary on
`cgr.dev/chainguard/static`, non-root, multi-arch — requires the
[ko CLI](https://ko.build/install/)):

```bash
ko build -B --local -t dev ./cmd/costguard   # → ko.local/costguard:dev
```

`-B` pins the image name to `.../costguard` (without it, ko appends an MD5
hash of the import path).

**Dry-run (warning only — the safe default):**

```bash
docker run --rm \
  -e STACKIT_SERVICE_ACCOUNT_KEY_PATH=/run/secrets/sa-key.json \
  -v "$PWD/sa-key.json:/run/secrets/sa-key.json:ro" \
  -e COSTGUARD_SCOPE=organisation \
  -e COSTGUARD_ORG_ID=00000000-0000-0000-0000-000000000000 \
  -e COSTGUARD_REGIONS=eu01 \
  -e COSTGUARD_MAX_AGE_DAYS=90 \
  -e COSTGUARD_SNA_MAX_AGE_DAYS=60 \
  -e COSTGUARD_SAFE_LABEL_KEY=do-not-delete \
  -e COSTGUARD_SAFE_LABEL_VALUE=true \
  -e COSTGUARD_OUTPUT=googlechat \
  -e COSTGUARD_WEBHOOK_URL='https://chat.googleapis.com/v1/spaces/.../messages?key=...&token=...' \
  ko.local/costguard:dev scan
```

**Execution mode (actually deletes due candidates):**

```bash
docker run --rm \
  ... same environment ... \
  -e COSTGUARD_DRY_RUN=false \
  ko.local/costguard:dev delete
```

**Callback server (stage 2):**

```bash
docker run --rm -p 8080:8080 \
  ... same environment ... \
  -e COSTGUARD_CALLBACK_URL=https://costguard.example \
  -e COSTGUARD_CALLBACK_SECRET="$(openssl rand -hex 32)" \
  -e COSTGUARD_WHITELIST_SECRET_PATH=secret/costguard/whitelist \
  -e COSTGUARD_SM_URL=https://secretsmanager.eu01.onstackit.cloud/INSTANCE_ID \
  -e COSTGUARD_SM_USERNAME=costguard-whitelist-writer \
  -e COSTGUARD_SM_PASSWORD='***' \
  ko.local/costguard:dev callback
```

Exit codes: `0` success, `1` fatal run error, `2` usage error.

## 6. Running with Kubernetes (recommended, two-stage)

The manifests in `deploy/` implement the two-stage rollout. Create the
namespace and the three credential Secrets first (see the header comments in
each manifest), then:

The three manifests pin the image
`professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/costguard:v0.0.1`,
built with ko and pushed to the internal registry on every `v*` tag push.
If you can't reach that registry (e.g. a customer cluster), push the image to
a registry you control (GHCR or your own) and update the `image:` field in all
three manifests — keeping the scan and delete jobs on the same version.

**Stage 1 — report-only (observe before you can delete):**

```bash
kubectl apply -n costguard -f deploy/cronjob-scan.yaml
```

A single **weekly** CronJob (`scan`, dry run): scans, writes mark labels to
new candidates (which also live-tests the label APIs), sends the report.
Nothing can be deleted in this stage.

**Stage 2 — delete mode:**

```bash
kubectl apply -n costguard -f deploy/cronjob-delete.yaml
kubectl apply -n costguard -f deploy/deployment-callback.yaml
# then set COSTGUARD_DRY_RUN=false in cronjob-delete.yaml and re-apply
```

- `cronjob-delete.yaml`: **weekly**, one hour after the scan. The offset plus
  the mark deadline guarantee that nothing first *seen* by this week's scan
  is deleted by this week's delete run — there is no hand-off state between
  the two jobs; the mark label in STACKIT is the only shared state.
- `deployment-callback.yaml`: stateless 2-replica Deployment + ClusterIP
  Service serving `/protect` and `/healthz`; front it with the customer's
  ALB/ingress so the button URLs are publicly reachable.
- Both CronJobs set `concurrencyPolicy: Forbid`, `activeDeadlineSeconds:
  3600`, and `successfulJobsHistoryLimit: 3`; every secret is referenced from
  a Secret, never a literal.

## 7. Prometheus output

Prometheus output is not supported in this version. Setting
`COSTGUARD_OUTPUT=prometheus` is rejected at startup with an actionable error.

## 8. Interactive buttons setup

Buttons are **signed links — no bot/app registration is needed** in any
platform:

- **Google Chat & Slack**: the report renders a *"Do not delete: …"* link
  (Slack uses `mrkdwn` link text). Clicking performs a `GET` to
  `COSTGUARD_CALLBACK_URL/protect?id=<resource>&type=<kind>&project=<p>&region=<r>&exp=<unix>&sig=<hmac>`.
- **Teams**: the same request is issued by a native Adaptive-Card
  `Action.Http` POST button.

The signature is HMAC-SHA256 (secret = `COSTGUARD_CALLBACK_SECRET`) over
`id|type|exp|project|region`. The callback verifies the signature, rejects
expired or forged requests, checks the resource still exists, and appends it
to the shared whitelist in the dedicated Secrets Manager instance using a
KV v2 **compare-and-swap** (concurrent clicks never lose entries; re-clicking
is idempotent). Button URLs expire at the candidate's deletion deadline;
clicks afterwards (or on already-deleted resources) get `410 Gone`.

Configure: set `COSTGUARD_CALLBACK_URL` + `COSTGUARD_CALLBACK_SECRET` on the
scan/delete workloads, and run the callback Deployment (section 6). Note the
HMAC stops *forged or stale* requests; room membership is the authorization
model — only put the report in rooms whose members may protect resources.

## 9. Protecting resources

Two mechanisms, checked on every scan and re-check before every deletion:

1. **Safe label** (per-resource, on the STACKIT resource itself):
   ```
   costguard-safe-label-key=do-not-delete, costguard-safe-label-value=true
   ```
   i.e. set the label `do-not-delete=true` (or your configured
   key/value) on the project, folder, network area, volume, or public IP.
2. **Whitelist** (three merged sources): the `whitelist:` YAML section, the
   `COSTGUARD_WHITELIST_*` env CSVs, and the shared Secrets Manager entry
   (populated by the buttons). Example YAML:

   ```yaml
   whitelist:
     projects: [11111111-1111-1111-1111-111111111111]
     folders: [22222222-2222-2222-2222-222222222222]
     volumes: [33333333-3333-3333-3333-333333333333]
     publicIps: [44444444-4444-4444-4444-444444444444]
   ```

   Protected projects/folders are *invisible* in the report; protected
   volumes/IPs simply never become candidates.

## 10. Cost anomaly detection

The bot pulls the last 30 days of per-day billing data (Cost API, day
granularity) and compares the **last 7-day average** spend against the
**preceding 7-day average**:

```
anomaly if last7_avg > prev7_avg * (1 + COSTGUARD_COST_ANOMALY_THRESHOLD_PCT / 100)
```

With the default threshold of `20`, a week that spends 20 % more than the
week before flags an anomaly line in the report (the percentage is shown).
Tune `COSTGUARD_COST_ANOMALY_THRESHOLD_PCT` to your workload's seasonality:
higher for spiky environments (e.g. 50), lower for flat production estates
(e.g. 10). The 30-day window is fixed — anomaly detection never uses data
older than that.

## 11. Dry-run vs execute

| | `COSTGUARD_DRY_RUN=true` (default) | `COSTGUARD_DRY_RUN=false` |
|---|---|---|
| Scan + mark labels | yes | yes |
| Report | yes, framed as *dry run — nothing will be deleted* | yes |
| `DELETE` calls | **never** | yes — mark-expired public IPs and detached volumes only |
| Confirmation message | no | yes, per-resource result (deleted / failed / skipped) |
| Re-validation before delete | n/a | yes, fail-closed (still idle/detached? mark expired? still unprotected?) |

Dry run is the safe default because the first weeks are for *observation*:
you want to see exactly what the bot would delete, and grant protections via
buttons/labels, before deletion becomes possible. The `delete` subcommand
honours the flag too — with dry run on it behaves like `scan` (global kill
switch).

## 12. Development

```bash
make build      # go build -o dist/costguard ./cmd/costguard
make test       # go test -race ./... with a >=80% per-package coverage gate (cmd/ exempt)
make lint       # go vet ./...
ko build -B --local -t dev ./cmd/costguard   # local image via ko (requires the ko CLI)
```

Coverage per package is printed by `make test` (profile under `coverage/`);
the gate fails the build if any package under `internal/` drops below 80 %.
Unit tests never touch real APIs: the STACKIT SDK surface is wrapped behind
interfaces in `internal/stackit`, and webhook/Secrets-Manager/S3 clients are
exercised against `httptest` servers.

---

*costguard is maintained as part of the [STACKIT professional-service
best-practice library](https://github.com/stackitcloud/professional-service).*
