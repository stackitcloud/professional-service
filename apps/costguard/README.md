<!-- tags: cost, cleanup, budgets, automation, iaas, terraform -->

# costguard

**costguard keeps your STACKIT cloud tidy and installs with one `terraform apply`.** It looks for things that cost
money but are not used by anything, reports them in a chat channel and, once you switch deletion on, deletes them.
It also watches monthly budgets and posts when one passes a threshold. It runs on a small server that logs in with
the service account attached to it, so no key is stored on the server.

You do not need to be a cloud expert to use it. This guide walks you through everything, step by step.

- [What costguard does](#what-costguard-does)
- [The two labels your team needs to know](#the-two-labels-your-team-needs-to-know)
- [What you need before you start](#what-you-need-before-you-start)
- [Install, step by step](#install-step-by-step)
- [Reading the messages](#reading-the-messages)
- [Turning on automatic deletion](#turning-on-automatic-deletion)
- [Budgets](#budgets)
- [Stopping, upgrading, removing](#stopping-upgrading-removing)
- [How costguard keeps you safe](#how-costguard-keeps-you-safe)
- [Security](#security)
- [Settings](#settings)
- [What it costs](#what-it-costs)
- [Questions and problems](#questions-and-problems)
- [Developing](#developing) and [Releasing](#releasing)

## What costguard does

Cloud resources are easy to create and easy to forget. A disk that is no longer attached to any server, or a public
IP address that nobody uses, keeps costing money every month. costguard finds them, tells your team in a chat channel
and, once you switch deletion on, deletes them.

It has three **features**, each with its own switch and schedule:

| Feature   | Default                      | What it does                                                                                                                                                                        |
| --------- | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `report`  | on, Monday 08:00             | Posts what could be cleaned up. **Changes nothing** while `delete` is off. With `delete` on, it also labels the new finds `delete=true`, so everybody gets a day to object.         |
| `delete`  | off (Tuesday 08:00 when on)  | Deletes what is labelled `delete=true`, unless it is protected or in use again, and posts what it deleted and what that saves. Needs `report`, and runs at least 20 hours after it. |
| `budgets` | off (Monday to Friday 10:00) | Posts every monthly budget whose spending so far is at or above a threshold, for example 80 % or 100 %.                                                                             |

Times are in `Europe/Berlin` unless you set another `time_zone`.

### What it looks for

| What                                           | Why it costs money                                         | What costguard does                                         |
| ---------------------------------------------- | ---------------------------------------------------------- | ----------------------------------------------------------- |
| **Unused public IP addresses**                 | A reserved IP address is billed even when nothing uses it. | Cleans it up (with `delete` on), up to 10 new ones per run. |
| **Disks (volumes) not attached to any server** | Storage is billed per GB, used or not.                     | Cleans it up (with `delete` on), up to 10 new ones per run. |
| **Anything your team labelled `delete=true`**  | –                                                          | Deletes it (with `delete` on).                              |
| **Disks that have snapshots**                  | Snapshots are often a deliberate backup.                   | Only lists them. Never labels them itself.                  |
| **Empty projects older than 30 days**          | Clutter, and a sign that something was forgotten.          | Only warns. Never deletes projects.                         |
| **Empty network areas older than 30 days**     | Clutter.                                                   | Only warns (when it looks at the whole organization).       |

An **empty project** has had no costs in the last 30 days and holds no servers, disks, IP addresses, Kubernetes
clusters or buckets. Projects that deliberately hold only service accounts, DNS zones or network settings show up
too; label them `do-not-delete=true` to silence the warning (costguard then ignores the project completely).

Every message says how many resources the next delete run deletes and how much money that saves per month and per
year. The amount is an estimate for IP addresses and disks, using the prices in [Settings](#settings); other
resources, such as servers, are counted but not priced.

## The two labels your team needs to know

A **label** is a small name tag on a resource in STACKIT, written as `name=value`. costguard understands exactly two:

| Label                | Meaning                                                                                                                                                                                                         |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `do-not-delete=true` | **Keep this.** costguard never deletes it. On a folder or project, costguard ignores everything inside. It always wins.                                                                                         |
| `delete=true`        | **Delete this at the next delete run.** costguard sets it on the things it found. You can also set it yourself on a server, disk, IP address, snapshot, network interface or security group to have it removed. |

That is all your colleagues need to remember: **if costguard lists something you still need, add
`do-not-delete=true` to it** (or remove `delete=true`). Labels can be set in the STACKIT portal, in Terraform, or with
the [STACKIT CLI](https://github.com/stackitcloud/stackit-cli):

```bash
stackit volume update <volume ID> --project-id <project ID> --labels do-not-delete=true
stackit public-ip update <IP ID> --project-id <project ID> --labels do-not-delete=true
stackit project update --project-id <project ID> --label do-not-delete=true
```

The same `--labels` flag exists for `server`, `security-group` and `network-interface` (which also needs
`--network-id`).

## What you need before you start

- [ ] **A STACKIT organization and its ID** (a UUID such as `a1b2c3d4-…`). After `stackit auth login`,
      `stackit organization list` shows it.
- [ ] **A deployer service account** that is **owner of the organization**, and a key file for it. Terraform uses
      it to create costguard's project, its service account and two custom roles on the organization. See
      [The deployer](#the-deployer) below.
- [ ] **Terraform 1.9 or newer**, or OpenTofu, and **git**.
- [ ] **A chat channel** in Microsoft Teams, Slack or Google Chat, and the right to add a webhook to it:

  - **Teams:** in the channel, open _Workflows_ and use the template _"Post to a channel when a webhook request is
    received"_; copy the URL it shows at the end.
  - **Slack:** create an _incoming webhook_ for the channel ([Slack guide](https://api.slack.com/messaging/webhooks)).
  - **Google Chat:** in the space, open _Apps & integrations_, then _Webhooks_, and add one.

  Treat the webhook URL like a password: anyone who has it can post into your channel.

- [ ] **About half an hour** for the first install.

costguard runs **once per organization**: its role names (`costguard.reader`, `costguard.cleaner`) are fixed.

### The deployer

Service accounts live inside a project, so the deployer needs a small admin project of its own. Nobody else should be
a member of it. With your personal login (`stackit auth login`), once:

```bash
ORG=<organization ID>
stackit project create --parent-id "$ORG" --name iac-admin          # note the project ID it prints
stackit service-account create --name deployer --project-id <iac-admin project ID>
stackit organization member add <deployer email> --organization-id "$ORG" --role owner
```

Then create a short-lived key for each install or change, and delete it afterwards:

```bash
stackit service-account key create --email <deployer email> --project-id <iac-admin project ID> \
  --expires-in-days 1 -o json -y > ~/.stackit/deployer-key.json
chmod 600 ~/.stackit/deployer-key.json
```

Why owner: whoever may create custom roles and role assignments on the organization can give itself any right
anyway, so a smaller role would not make the deployer less powerful. Keep its key short-lived instead. Landing zone
customers often have such an admin project already.

## Install, step by step

1. **Get the code of a release.** Releases are the `apps/costguard/v…` entries on the
   [releases page](https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases);
   always install from such a tag, never from `main`:

   ```bash
   git clone --depth 1 --branch apps/costguard/<version> \
     https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service.git
   cd professional-service/apps/costguard/terraform
   ```

2. **Fill in your settings.** Copy the example and edit it:

   ```bash
   cp terraform.tfvars.example terraform.tfvars
   chmod 600 terraform.tfvars
   ```

   You need at least `service_account_key_path` (the deployer's key file), `project_owner_email` (the deployer's
   email), `organization_id`, `output` (`teams`, `slack` or `googlechat`) and `webhook_url`. Nothing needs to be
   exported. **Never commit `terraform.tfvars`**: it holds the webhook URL (git ignores it in this repository).

   For the first install, leave `delete` off. Start with `report` only, and `budgets` if you want them.

3. **Install:**

   ```bash
   terraform init
   terraform plan -out=tfplan
   terraform apply tfplan
   rm tfplan    # a saved plan holds the webhook URL in plain text
   ```

   The plan shows about 11 resources to add and one warning: the role assignments use the provider's `iam`
   experiment (see [Security](#security)). The apply takes about 5 minutes.

4. **Wait for the first message** (a few minutes after the apply). It is titled _"costguard: vX.Y.Z is running (<your organization>)"_ and
   proves that the login, the roles and the chat work. It contains the first report, where every budget stands, and
   when the next runs are. Nothing is changed by it.

Every time the server is created or replaced (every settings change replaces it), it posts this message again.

## Reading the messages

- The **title** says what the message is (_report_, _deletion_, _budget_, …) and names your organization, for example
  _"costguard: report (Acme GmbH)"_; the line below it says what was looked at (the scope), the regions and the time.
- Each **list** shows one line such as _"Idle public IPs: 3, about €8,76/month"_. In Google Chat and Teams, _Show
  more_ unfolds the entries; each entry names the resource, its project and region, and links to its page in the STACKIT
  portal. Slack shows the entries right away.
- **Everything that the next delete run deletes is always listed**, including everything someone labelled
  `delete=true`.
- **New finds are limited to 10 per category per run**, the biggest savings first, and costguard only labels the
  ones it lists. If it found more, the list says how many are waiting; they come up in the following runs.
- **"Could not be read"** lists what costguard was not allowed to read or what did not answer. It leaves those parts
  alone.
- The footer says how many folders and projects were skipped, and the costguard version.

Let the report run for a few weeks. Each time, look at the list and **add `do-not-delete=true` to everything that
must stay**. When the list only contains things you really want gone, you are ready for deletion.

**No message on a report day means something is wrong**: check the server (see
[Questions and problems](#questions-and-problems)). With only `budgets` on there is no weekly message, so silence
is normal there.

## Turning on automatic deletion

Before you switch:

- [ ] You have read a few reports, and everything that must stay has `do-not-delete=true` or is in `skip`.
- [ ] Your colleagues know the two labels, and they know that the report is their chance to object.
- [ ] Nobody else in your company uses a label called `delete` for something else.

Then, **at least a day before the next delete run**, add to `terraform.tfvars`:

```hcl
features = {
  delete = { enabled = true }   # Tuesday 08:00; the report stays on Monday 08:00
}
```

and run `terraform apply`. This creates the `costguard.cleaner` role (the right to change labels and delete) and
replaces the server. Its first message lists everything **already** labelled `delete=true`: those go at the next
delete run, so read it right away. New finds are labelled at the next report run and deleted at the delete run after
that.

**Other schedules.** Each feature takes `days` (`Mon` … `Sun`) and a `time` (`HH:MM`). Every delete run must come
at least 20 hours after the report run before it; Terraform refuses anything else. To clear a big backlog faster (10
new finds per category per run), run both every working day, the delete run first:

```hcl
features = {
  report = { days = ["Mon", "Tue", "Wed", "Thu", "Fri"], time = "08:00" }
  delete = { enabled = true, days = ["Mon", "Tue", "Wed", "Thu", "Fri"], time = "07:30" }
}
```

Every morning the delete run then deletes what the report run labelled the working day before, 23½ hours earlier.
Avoid 03:30 to 04:30: the server installs security updates and reboots at 04:00 when needed.

## Budgets

```hcl
features = {
  budgets = {
    enabled    = true
    days       = ["Mon", "Tue", "Wed", "Thu", "Fri"]   # the default
    time       = "10:00"
    thresholds = [80, 100]                              # percent of the limit; the default
    limits = [
      { name = "Whole organization", organization = true, monthly_eur = 20000 },
      { name = "Team A", folder = "team-a", monthly_eur = 2500 },
      { name = "Sandbox X", project = "<project ID>", monthly_eur = 50, thresholds = [50, 100] },
    ]
  }
}
```

- Each limit has exactly one target: the whole organization, a folder (every project below it) or a project, by ID
  or exact name.
- Each budgets run posts **every budget whose spending this month is at or above one of its thresholds**: the
  amount so far against the limit (_"Team A: €2.034,12 of €2.500,00 in September (81%)"_), a forecast for the month
  (from the 7th on), and for organization and folder budgets the five projects that spent the most, numbered. It posts again at every run until the month ends; with nothing at
  or above a threshold, it posts nothing.
- The amounts are the ones the STACKIT cost dashboard shows. Months are calendar months in UTC. STACKIT has a day's
  costs after 07:30 UTC the next day, so an earlier run sees them a day later.
- **The 1st of a month posts nothing**: none of that month's costs are in yet. A threshold that is first reached on
  the last day of a month is therefore never posted.
- Budgets ignore `scope` and `skip`: those protect resources from deletion and must not hide spending.
- A folder budget counts the projects that are in the folder now; projects moved away or deleted during the month
  no longer count for it.

## Stopping, upgrading, removing

**Stop it immediately** (for example if a message looks wrong): stop the server.

```bash
stackit server stop $(terraform output -raw server_id) --project-id $(terraform output -raw project_id)
```

Nothing runs while it is stopped. It stays stopped until you start it again (`stackit server start …`) or an apply
replaces it (every settings change does); after a start it continues with its schedule without a new first message.

**Stop deleting for good:** set `delete = { enabled = false }` and apply. This removes the delete role and the delete
run. The `delete=true` labels stay on the resources.

**Upgrade** to a newer release:

```bash
git fetch --depth 1 origin tag apps/costguard/<new version>
git checkout apps/costguard/<new version>
terraform init
terraform apply
```

Keep your `terraform.tfvars` and the state (`terraform.tfstate`) in place. The server is replaced and posts its
first message with the new version.

**Remove costguard completely:** `terraform destroy`. The labels it set stay on the resources.

## How costguard keeps you safe

- **Without `delete`, nothing is changed.** Its service account then has no right to change anything.
- **Everybody gets a day's notice.** Things costguard finds itself are announced by the report run and deleted by a
  delete run at least 20 hours later.
- **`do-not-delete=true` always wins,** also on whole folders and projects.
- **A forgotten `delete=true` on a protected resource is cleaned up.** If a disk or IP address carries both labels,
  the report run removes the old `delete=true`, so lifting the protection later cannot delete it by surprise. Other
  resources with both labels are listed, and the message asks you to remove the `delete` label by hand.
- **Nothing is deleted that was not in a message.** Everything labelled `delete=true` is always listed, and costguard
  only labels the new finds it lists. The one exception: a `delete=true` someone adds after the report run (see
  below).
- **Everything is checked again right before it is deleted.** If someone added `do-not-delete=true`, removed
  `delete=true` or started using the resource again in the meantime, it stays.
- **Disks and IP addresses in use are never deleted,** even when labelled. If one is used again, costguard removes its
  label.
- **It only deletes the labelled item itself.** Deleting a server does not delete its unlabelled disks; they become
  finds of the next report run.
- **It never deletes projects, folders or network areas**, and never an IP address that a load balancer uses.
- **It never labels disks that have snapshots, or disks in projects that run Kubernetes (SKE)** by itself. They are
  only deleted if someone labels them `delete=true` on purpose.
- **When in doubt, it does nothing.** If costguard cannot read something, it leaves it alone and says so.
- **No message, no labels.** If the report cannot be delivered, nothing new is labelled.
- **Corrections go to the chat too.** If a label cannot be written after the report went out, costguard posts a
  correction that lists what will _not_ be deleted.
- **A broken skip list stops everything.** If an entry in `skip` matches nothing (for example because a project was
  renamed), costguard labels and deletes nothing until you fix it, and says so.
- **costguard protects itself:** its project and resources carry `do-not-delete=true`.

### Please be aware

- **`delete=true` means "delete", no matter who set it or when.** If someone adds it after the report run, the item
  is deleted at the next delete run without having been listed.
- **Anyone who can change labels on a resource can have costguard delete it.** Keep permissions tidy.
- **Images are not handled.** costguard neither lists nor deletes images; a `delete=true` on an image does nothing.
- **There is no limit on how much one delete run deletes.** Read the report, especially the first one with delete
  on.
- **IP addresses held by other STACKIT services.** An IP address that is attached to no network interface and used
  by no load balancer counts as unused. Whether the addresses of VPN gateways and other managed services look like
  that is not verified yet. Until it is, protect such addresses with `do-not-delete=true` or keep those projects in
  `skip`.
- **Before you lift a skip** (remove a folder or project from `skip`, or its `do-not-delete`), check it for old
  `delete=true` labels. costguard does not look inside skipped folders and projects; after the skip is lifted, such
  resources are listed at the next report run and deleted at the delete run after it.
- **If a great many resources are labelled at once** (someone labels 300 servers), the message can get too big for
  the chat and not arrive. They are still deleted. Label large amounts in batches.

## Security

- **No key anywhere on the server.** The server gets short-lived tokens (one hour) for the service account attached
  to it from STACKIT's metadata service. The deployer's key only lives on your machine.
- **Nobody can log in to the server:** no SSH, no password, no inbound firewall rule, no public IP, and no STACKIT
  Server Agent (it would run commands sent through the API as root).
- **A project of its own.** Terraform creates the project `costguard` directly under the organization, with the
  deployer as its only owner. Whoever is owner, editor or service account user on that project, or on a folder above
  it, can act as costguard's service account, so keep it that way (Terraform warns if you choose a folder as parent).
- **Rights on the whole organization.** costguard's service account has the custom role `costguard.reader` (read
  resources and costs, 24 permissions) and, only while `delete` is on, `costguard.cleaner` (change labels, delete
  six resource types, 8 permissions), both on the organization. `scope` and `skip` limit what costguard _does_,
  not what its service account _may_ do. The permissions are listed in
  [`terraform/060-identity.tf`](terraform/060-identity.tf).
- **The webhook URL is stored in the Terraform state and in the server's user data.** The user data can be read by
  anyone who may read server details in costguard's project. If it leaks, someone can post into your channel: create
  a new webhook and apply.
- **Terraform experiments:** the role assignments need the provider's `iam` experiment ("unstable features without
  official support") and the image lookup a beta data source. Check the plan before you upgrade the provider
  (`.terraform.lock.hcl` pins the tested version).
- **The binary:** Terraform reads its SHA-256 hashes from the release's `SHA256SUMS` when it plans, and the server
  installs only a binary with that hash. The `binary` output shows them. The build is reproducible, so you can
  compare them with your own build ([Releasing](#releasing)), or pin hashes you checked yourself with
  `binary_override`.
- **Audit log:** every deletion by costguard is in the audit log of the project it happened in (portal: _Information
  → Audit log_, kept for 90 days). The audit log is kept per project; to collect all of them in one place, route them
  with the Telemetry Router
  ([example](../../examples/telemetry-router-hub-spoke-setup)).
- **Terraform state:** local by default. Whoever has the state file controls the install, and every later change
  needs it. For production, keep it in STACKIT Object Storage:

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

  Object Storage offers no state locking; if several people apply, use the
  [PostgreSQL backend](../../examples/terraform-pg-backend-state-locking) instead.

## Settings

Everything goes into `terraform.tfvars`; [`terraform.tfvars.example`](terraform/terraform.tfvars.example) shows each
setting. Misspelled keys in `features`, `scope` and `skip` are refused, not ignored.

| Setting                                       | Default                                 | What it does                                                                                                                                                                |
| --------------------------------------------- | --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `service_account_key_path`                    | `STACKIT_SERVICE_ACCOUNT_KEY_PATH`      | The deployer's key file.                                                                                                                                                    |
| `project_owner_email`                         | – (required for a new project)          | The deployer's email; becomes owner of costguard's project.                                                                                                                 |
| `organization_id`                             | – (required)                            | Your organization.                                                                                                                                                          |
| `output`, `webhook_url`                       | – (required)                            | The chat (`teams`, `slack` or `googlechat`) and its webhook. Terraform warns if the URL does not look like that chat's.                                                     |
| `features`                                    | report on Monday 08:00                  | See [What costguard does](#what-costguard-does), [Turning on automatic deletion](#turning-on-automatic-deletion) and [Budgets](#budgets).                                   |
| `time_zone`                                   | `Europe/Berlin`                         | Time zone of the schedules, the messages and the server.                                                                                                                    |
| `scope`                                       | the whole organization                  | `{ folders = [...], projects = [...] }`: only look at these, by ID or exact name. Each entry must match exactly one folder or project.                                      |
| `skip`                                        | none                                    | `{ folders = [...], projects = [...] }`: never look at these. A name used several times skips all of them; an entry that matches nothing blocks all labelling and deleting. |
| `regions`                                     | `["eu01"]`                              | **List every region your company uses**; others are not looked at.                                                                                                          |
| `warn_empty_after_days`                       | `30`                                    | Age of an empty project or network area before costguard warns; `0` switches these warnings off.                                                                            |
| `prices`                                      | 2.92 € per IP, 0.065 € per GB and month | `{ public_ip_monthly_eur, volume_gb_monthly_eur }` for the savings estimate (net, STACKIT price list).                                                                      |
| `project_id`, `network_id`                    | a new project and network               | Use an existing project (for example a landing zone tooling project) or network (for example one in a STACKIT Network Area).                                                |
| `parent_container_id`                         | the organization                        | Where the new project is created. A folder works, but its admins could then act as costguard.                                                                               |
| `region`, `availability_zone`, `machine_type` | `eu01`, `eu01-1`, `t2i.1`               | Where the server runs and its size.                                                                                                                                         |
| `log_level`                                   | `info`                                  | costguard's log on the server.                                                                                                                                              |
| `binary_override`, `download_url`             | the release this code pins              | Only for test builds or binaries you host yourself; see [Releasing](#releasing).                                                                                            |

## What it costs

About **€15.50 per month** (STACKIT price list, net): the server `t2i.1` €10.18, its 10 GB disk €2.42, and the
public IP of the new network's router €2.92 (none with an existing `network_id`).

## Questions and problems

**No message after the install.** Wait five minutes: the first run waits for the login. Then look at the server's
console, which shows the setup and the download of costguard (never the webhook URL or a token):

```bash
stackit server log $(terraform output -raw server_id) --project-id $(terraform output -raw project_id)
```

If costguard could not be downloaded, the server tries four times over about 15 minutes and then posts _"costguard
vX.Y.Z could not be installed …"_ with the reason. If nothing arrives at all, the chat settings are wrong (`output`
or `webhook_url`; the plan warns when they don't match).

**No message on a report day.** Check that the server runs (`stackit server describe …`). Runs that were missed
because the server was down are not caught up; the next scheduled run works normally.

**"costguard cannot log in to STACKIT".** The service account is not attached to the server. Run `terraform apply`
again.

**"costguard cannot read the organization".** The `costguard.reader` role is missing on the organization. Run
`terraform apply` again.

**Something "could not be read".** costguard may not read part of your organization, or a service did not answer.
It leaves that part alone until it can read it.

**"deletions blocked".** An entry in `skip` matches nothing inside the scope, usually because a folder or project was
renamed or deleted. Fix it in `terraform.tfvars` and apply.

**A scope entry matches nothing or several things.** Use the exact name, or better the ID.

**Something was listed that we still need.** Add `do-not-delete=true` to it before the next delete run, or remove its
`delete=true`.

**Something was deleted that we still needed.** costguard cannot restore it. The delete run's message and the
project's audit log say what was deleted and when; protect similar resources with `do-not-delete=true`.

**A run stopped at 04:00.** The server rebooted after security updates; move the run out of 03:30 to 04:30.

**Does costguard delete projects or folders?** No. It only warns about empty projects and network areas.

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
   that is released already; a release that failed needs a new version.
3. `terraform plan` reads the hashes from that `SHA256SUMS` (so it needs the release to exist and Forgejo to be
   reachable) and shows them in the `binary` output; the server installs only a binary with that hash. The build is
   reproducible: `make dist VERSION=vX.Y.Z` at the tag builds the same files, if you want to compare.

`main` keeps the latest release's version until the next one, so a checkout of `main` installs the last release's
binary with possibly newer Terraform code: install from a tag. Test builds set `binary_override` (with their own
hashes; nothing is read at plan time) and `download_url` (see
[`terraform.tfvars.example`](terraform/terraform.tfvars.example)); to host a release yourself, put the files of
`make dist` on any HTTPS address and set `download_url` to it.
