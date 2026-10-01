<!-- tags: cost, cleanup, budgets, automation, iaas, terraform -->

# costguard

**costguard keeps your STACKIT cloud tidy and installs with one `terraform apply`.** It looks for things that cost
money but are not used by anything, reports them in a chat channel and, once you switch deletion on, deletes them.
It also watches monthly budgets and posts when one passes a threshold. It runs on a small server that logs in with
the service account attached to it, so no key is stored on the server.

- [What it does](#what-it-does)
- [The two labels](#the-two-labels)
- [Before you start](#before-you-start)
- [Install](#install)
- [Turning on deletion](#turning-on-deletion)
- [Budgets](#budgets)
- [Stop, upgrade, remove](#stop-upgrade-remove)
- [Safety](#safety)
- [Security](#security)
- [Troubleshooting](#troubleshooting)

## What it does

| Feature   | Default                      | What it does                                                                                         |
| --------- | ---------------------------- | ---------------------------------------------------------------------------------------------------- |
| `report`  | on, Monday 08:00             | Posts what could be cleaned up.                                                                      |
| `delete`  | off (Tuesday 08:00 when on)  | Deletes what is labelled `delete=true`, except protected resources and disks or IP addresses in use. |
| `budgets` | off (Monday to Friday 10:00) | Posts every monthly budget whose spending so far is at or above a threshold.                         |

Times are in `time_zone` (default `Europe/Berlin`). Every setting, with its default, is in
[`terraform.tfvars.example`](terraform/terraform.tfvars.example).

The report looks at every folder and project in `scope` (default: the whole organization) except those in `skip`, in
the regions in `regions` (default: only `eu01`, so list every region you use):

| Finding                                              | What happens                                                 |
| ---------------------------------------------------- | ------------------------------------------------------------ |
| Public IP addresses that nothing uses                | Listed; with `delete` on, labelled `delete=true` and deleted |
| Disks (volumes) attached to no server                | Listed; with `delete` on, labelled `delete=true` and deleted |
| Anything labelled `delete=true`                      | Listed; with `delete` on, deleted at the next delete run     |
| Disks that have snapshots                            | Only listed                                                  |
| Empty projects and network areas, older than 30 days | Only a warning                                               |

Each message says what every entry costs per month and how much the next delete run saves. The prices come from
STACKIT's public price list ([PIM API](https://pim.api.stackit.cloud/v2/skus)) at every run: list prices, net, without
contract discounts or OS licences; a deallocated server counts as €0. If the price list can't be read, the message
shows no amounts and says so. Each list folds out (_Show more_ in Google Chat and Teams) into entries that link to the
resource in the STACKIT portal.
costguard ignores images and networks (a `delete=true` on them does nothing) and never labels disks in projects that
run Kubernetes (SKE) by itself. An empty project has had no costs for 30 days and holds no servers, disks, IP addresses,
SKE clusters or buckets; `do-not-delete=true` on it silences the warning.

### The report run, with and without `delete`

- **`delete` off:** the report run only reads and posts. costguard's service account has no right to change anything.
- **`delete` on:** the report run posts first, then labels the new finds it listed `delete=true`: only unused IP
  addresses and detached disks, at most 10 of each per run, the biggest first. It never labels servers, snapshots,
  network interfaces or security groups; those are only deleted when a person labels them. It also removes
  `delete=true` from disks and IP addresses that are in use again or carry `do-not-delete=true`.
- **No message, no labels:** if the message cannot be delivered, nothing is labelled. If a label cannot be written
  after the message went out, costguard posts a correction.
- The delete run then deletes, at least 20 hours later, everything labelled `delete=true` and posts what it deleted.

## The two labels

| Label                | Meaning                                                                                                                                                                 |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `do-not-delete=true` | **Keep this.** On a folder or project, costguard ignores everything inside. It always wins.                                                                             |
| `delete=true`        | **Delete this at the next delete run.** costguard sets it on its finds; anyone can set it on a server, disk, IP address, snapshot, network interface or security group. |

If costguard lists something you still need, add `do-not-delete=true` to it before the next delete run, with the
[STACKIT CLI](https://github.com/stackitcloud/stackit-cli):

```bash
stackit volume update <volume ID> --project-id <project ID> --labels do-not-delete=true
stackit public-ip update <IP ID> --project-id <project ID> --labels do-not-delete=true
stackit project update --project-id <project ID> --label do-not-delete=true
stackit curl -X PATCH https://resource-manager.api.stackit.cloud/v2/folders/<folder ID> \
  --data '{"labels": {"do-not-delete": "true"}}' --fail
```

`server`, `security-group` and `network-interface` (with `--network-id`) take `--labels` too. A project or folder
can only be labelled by someone who may edit it. If it is managed with Terraform, set the label in its `labels`
there, or the next apply removes it.

## Before you start

- **Your organization ID:** after `stackit auth login`, `stackit organization list` shows it.
- **A deployer service account that is owner of the organization,** with a short-lived key. Terraform uses it to
  create costguard's project, its service account and two custom roles on the organization. Service accounts live in
  a project, so give it a small admin project that nobody else is a member of (landing zone customers often have
  one):

  ```bash
  ORG=<organization ID>
  stackit project create --parent-id "$ORG" --name iac-admin    # note the project ID
  stackit service-account create --name deployer --project-id <iac-admin project ID>
  stackit organization member add <deployer email> --organization-id "$ORG" --role owner
  stackit service-account key create --email <deployer email> --project-id <iac-admin project ID> \
    --expires-in-days 1 -o json -y > ~/.stackit/deployer-key.json
  chmod 600 ~/.stackit/deployer-key.json
  ```

  A smaller role would not help: whoever may create and assign custom roles on the organization can give itself any
  right. Create a new key for each change instead and let it expire.

- **Terraform 1.9 or newer** (or OpenTofu) and **git**.
- **A chat webhook.** Treat it like a password: anyone who has it can post into the channel.
  - Teams: in the channel, _Workflows_, template _"Post to a channel when a webhook request is received"_.
  - Slack: an [incoming webhook](https://api.slack.com/messaging/webhooks).
  - Google Chat: in the space, _Apps & integrations_, then _Webhooks_.

costguard runs once per organization: its role names (`costguard.reader`, `costguard.cleaner`) are fixed. It costs
about €15.50 per month (net): the server `t2i.1` €10.18, its 10 GB disk €2.42 and the new network's router IP €2.92
(none with an existing `network_id`).

## Install

1. Clone a release, never `main`. Releases are the `apps/costguard/v…` tags on the
   [releases page](https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases).

   ```bash
   git clone --depth 1 --branch apps/costguard/<version> \
     https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service.git
   cd professional-service/apps/costguard/terraform
   ```

2. Fill in the required settings at the top. Leave `delete` off for now.

   ```bash
   cp terraform.tfvars.example terraform.tfvars
   chmod 600 terraform.tfvars    # it holds the webhook URL: never commit it
   ```

3. Apply:

   ```bash
   terraform init
   terraform plan -out=tfplan    # about 11 to add, and a warning about the "iam" experiment
   terraform apply tfplan
   rm tfplan                     # a saved plan holds the webhook URL
   ```

4. A few minutes later the chat shows _"costguard: vX.Y.Z is running (<your organization>)"_. It proves that login,
   roles and chat work, shows the first report, where each budget stands and when the next runs are, and changes
   nothing. The server posts it again whenever it is replaced, which every settings change does.

Let the report run for a few weeks and add `do-not-delete=true` to everything that must stay. **No message on a
report day means something is wrong** (see [Troubleshooting](#troubleshooting)); with only `budgets` on, silence is
normal.

## Turning on deletion

When the report only lists things you really want gone, and your colleagues know the two labels, add this to
`terraform.tfvars` **at least a day before the next delete run** and apply:

```hcl
features = {
  delete = { enabled = true }    # Tuesday 08:00; the report stays on Monday 08:00
}
```

The apply creates the `costguard.cleaner` role (change labels, delete) and replaces the server. Its first message
lists everything **already** labelled `delete=true`: that goes at the next delete run, so read it right away.

Each feature takes `days` (`Mon` … `Sun`) and a `time` (`HH:MM`). Every delete run must come at least 20 hours after
the report run before it; Terraform refuses anything else, including both at the same time. To work off a backlog
faster, run both every working day, the delete run first:

```hcl
features = {
  report = { days = ["Mon", "Tue", "Wed", "Thu", "Fri"], time = "08:00" }
  delete = { enabled = true, days = ["Mon", "Tue", "Wed", "Thu", "Fri"], time = "07:30" }
}
```

Each delete run then deletes what the report run labelled the working day before. Avoid 03:30 to 04:30: the server
installs security updates and reboots at 04:00 when needed.

## Budgets

Switch on `features.budgets` and add limits, as in the example. Each limit has one target: the whole organization, a
folder (every project below it) or a project, by ID or exact name.

- Each budgets run posts every budget whose spending this month is at or above one of its thresholds, with a forecast
  (from the 7th) and, for organization and folder budgets, the five projects that spent the most. It posts again at
  every run until the month ends; with nothing at or above a threshold, it posts nothing.
- The amounts are the ones the STACKIT cost dashboard shows, per calendar month in UTC. A day's costs come in after
  07:30 UTC the next day.
- **The 1st of a month posts nothing** (none of the month's costs are in yet), so a threshold first reached on the
  last day of a month is never posted.
- Budgets ignore `scope` and `skip`. A folder budget counts the projects that are in the folder now.

## Stop, upgrade, remove

- **Stop now:** stop the server. Nothing runs until you start it again or an apply replaces it; after a start it
  continues its schedule without a new first message.

  ```bash
  stackit server stop $(terraform output -raw server_id) --project-id $(terraform output -raw project_id)
  ```

- **Stop deleting:** set `delete = { enabled = false }` and apply. The delete role and the delete run are removed;
  the labels stay.
- **Upgrade:** keep `terraform.tfvars` and the state (`terraform.tfstate`) in place, then

  ```bash
  git fetch --depth 1 origin tag apps/costguard/<new version>
  git checkout apps/costguard/<new version>
  terraform init
  terraform apply
  ```

- **Remove:** `terraform destroy`. The labels costguard set stay on the resources.

## Safety

- **Nothing is deleted that was not in a message,** except a `delete=true` someone adds after the report run: that is
  deleted at the next delete run without being listed first.
- **Everything is checked again right before it is deleted.** If it got `do-not-delete=true` or lost `delete=true` in
  the meantime, it stays. Disks and IP addresses in use are never deleted, even when labelled.
- **Only the labelled item is deleted.** Deleting a server does not delete its unlabelled disks.
- **Never deleted:** projects, folders, network areas and IP addresses used by a load balancer.
- **When in doubt, it does nothing.** What costguard cannot read, it leaves alone and lists under _Could not be read_.
- **A broken skip list stops everything.** A `skip` entry that matches nothing (for example a renamed project) stops
  all labelling and deleting until you fix it.
- **costguard protects itself:** its project and resources carry `do-not-delete=true`.

Please be aware:

- `delete=true` means "delete", whoever set it. Anyone who can change labels on a resource can have costguard delete
  it, and nobody in your company should use a label called `delete` for something else.
- There is no limit on how much one delete run deletes. Read the first message with `delete` on carefully.
- Whether the IP addresses of VPN gateways and other STACKIT services count as unused is not verified yet. Protect
  them with `do-not-delete=true`, or keep those projects in `skip`.
- Before you lift a skip, look for old `delete=true` labels inside: they are listed at the next report run and
  deleted at the delete run after it.
- A message with hundreds of entries can be too big for the chat and not arrive. Label large amounts in batches.

## Security

- **No key on the server.** It gets one-hour tokens for its attached service account from STACKIT's metadata
  service and sends them only to STACKIT's APIs, never to the public price list. Nobody can log in: no SSH, no
  password, no public IP, no inbound rule and no STACKIT Server Agent.
- **A project of its own,** directly under the organization, with the deployer as its only owner. Whoever is owner,
  editor or service account user on it, or on a folder above it, can act as costguard; Terraform warns if you choose
  a folder as parent.
- **Rights on the whole organization:** the custom role `costguard.reader` (read resources and costs) and, only while
  `delete` is on, `costguard.cleaner` (change labels, delete). `scope` and `skip` limit what costguard does, not what
  it may do. The permissions are in [`terraform/060-identity.tf`](terraform/060-identity.tf).
- **The webhook URL** is in the Terraform state and in the server's user data, which anyone who may read server
  details in costguard's project can read. If it leaks, create a new webhook and apply.
- **The binary** is installed only if it matches the SHA-256 hashes Terraform reads from the release's `SHA256SUMS`
  when it plans (the `binary` output shows them). The build is reproducible: `make dist VERSION=<version>` at the tag
  builds the same files. To host them yourself, put them on any HTTPS address and set `download_url`;
  `binary_override` pins hashes you checked yourself.
- **Provider features:** the role assignments need the provider's experimental `iam` feature and the image lookup a
  beta data source. `.terraform.lock.hcl` pins the tested provider version; check the plan before you upgrade it.
- **Audit log:** each deletion is in the audit log of its project (portal: _Information → Audit log_, 90 days). To
  collect them in one place, use the [Telemetry Router](../../examples/telemetry-router-hub-spoke-setup).
- **Terraform state** is local by default, and whoever has it controls the install. For production, keep it in
  STACKIT Object Storage (no locking; for locking use the
  [PostgreSQL backend](../../examples/terraform-pg-backend-state-locking)):

  ```hcl
  terraform {
    backend "s3" {
      bucket                      = "<bucket>"
      key                         = "costguard/terraform.tfstate"
      region                      = "eu01"
      endpoints                   = { s3 = "https://object.storage.eu01.onstackit.cloud" }
      skip_credentials_validation = true
      skip_region_validation      = true
      skip_s3_checksum            = true
      skip_requesting_account_id  = true
    }
  }
  ```

## Troubleshooting

**No message after the install.** Wait five minutes: the first run waits for the login. Then read the server's
console, which shows the setup and the download (never the webhook URL or a token):

```bash
stackit server log $(terraform output -raw server_id) --project-id $(terraform output -raw project_id)
```

A failed download is retried for about 15 minutes, then posted with the reason. If nothing arrives at all, `output`
or `webhook_url` is wrong (the plan warns when they don't match).

**No message on a report day.** Check that the server runs. Missed runs are not caught up.

**"costguard cannot log in to STACKIT" or "cannot read the organization".** Run `terraform apply` again; it restores
the attached service account and the roles.

**"deletions blocked".** A `skip` entry matches nothing, usually because a folder or project was renamed or deleted.
Fix it and apply.

**A scope entry matches nothing or several things.** Use the exact name, or better the ID.

**Something was deleted that was still needed.** costguard cannot restore it. The delete run's message and the
project's audit log say what was deleted and when.

**A run stopped at 04:00.** The server rebooted after security updates; move the run out of 03:30 to 04:30.
