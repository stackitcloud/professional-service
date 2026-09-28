<!-- tags: cost, cleanup, automation, iaas, kubernetes -->

# costguard

**costguard keeps your STACKIT cloud tidy.** It looks for things that cost
money but are not used by anything, tells your team about them in a chat
channel every Monday and, once you are ready, deletes them for you.

You do not need to be a cloud expert to use it. This guide walks you through
everything, step by step.

- [What costguard does](#what-costguard-does)
- [The two labels your team needs to know](#the-two-labels-your-team-needs-to-know)
- [What you need before you start](#what-you-need-before-you-start)
- [Setup, step by step](#setup-step-by-step)
- [Turning on automatic cleanup](#turning-on-automatic-cleanup)
- [How costguard keeps you safe](#how-costguard-keeps-you-safe)
- [Settings](#settings)
- [Questions and problems](#questions-and-problems)

## What costguard does

Cloud resources are easy to create and easy to forget. A disk that is no
longer attached to any server, or a public IP address that nobody uses,
keeps costing money every month. costguard finds them.

It works in two stages. You start with stage 1 and switch to stage 2 when
you trust what it reports.

|                          | Every Monday at 08:00                                                              | Every Tuesday at 08:00                                                                                  |
| ------------------------ | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| **Stage 1: report only** | Posts a list of what could be cleaned up. **Nothing is changed.**                  | –                                                                                                       |
| **Stage 2: clean up**    | Marks new things to clean up and posts the list, so everybody has a day to object. | Deletes what was marked, unless someone said "keep it", and posts what was deleted and what that saves. |

### What it looks for

| What                                             | Why it costs money                                         | What costguard does                                   |
| ------------------------------------------------ | ---------------------------------------------------------- | ----------------------------------------------------- |
| **Unused public IP addresses**                   | A reserved IP address is billed even when nothing uses it. | Cleans it up (stage 2), up to 10 new ones per run.    |
| **Disks (volumes) not attached to any server**   | Storage is billed per GB, used or not.                     | Cleans it up (stage 2), up to 10 new ones per run.    |
| **Anything your team marked with `delete=true`** | –                                                          | Deletes it (stage 2).                                 |
| **Disks that have snapshots**                    | Snapshots are often a deliberate backup.                   | Only lists them. Never marks them itself.             |
| **Empty projects older than 30 days**            | Clutter, and a sign that something was forgotten.          | Only warns. Never deletes projects.                   |
| **Empty network areas older than 30 days**       | Clutter.                                                   | Only warns (when it looks at the whole organization). |

An **empty project** has had no costs in the last 30 days and holds no
servers, disks, IP addresses, Kubernetes clusters or buckets. Projects that
deliberately hold only service accounts, DNS zones or network settings show
up too; mark them `do-not-delete=true` to silence the warning (costguard then
ignores the project completely).

Every message says **how many resources that run deletes and how much money
that saves**:

- **Monday:** what will be deleted on Tuesday, for example _"Deleting these
  5 resource(s) saves about €19.28 per month (€231.36 per year)"_.
- **Tuesday:** what this run actually deleted, for example _"This run
  deleted 5 resource(s), saving about €19.28 per month (€231.36 per year)"_.

Deleting a resource ends its monthly bill, so the saving is shown per month
and per year. The amount is an estimate for IP addresses and disks, using the
prices in [Settings](#settings). Other resources, such as servers, are
counted in the number of deleted resources but not in the amount.

## The two labels your team needs to know

A **label** is a small name tag you can attach to a resource in STACKIT,
written as `name=value`. costguard understands exactly two:

| Label                | Meaning                                                                                                                                                                                                       |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `do-not-delete=true` | **Keep this.** costguard never deletes it. Put it on a folder or project and costguard ignores everything inside. It always wins.                                                                             |
| `delete=true`        | **Delete this on Tuesday.** costguard sets it on the things it found. You can also set it yourself on a server, disk, IP address, snapshot, image, network interface or security group to have it cleaned up. |

That is all your colleagues need to remember: **if costguard lists something
you still need, add `do-not-delete=true` to it** (or remove `delete=true`).
Labels can be set in the STACKIT portal, with the STACKIT command line tool,
or in Terraform, wherever your team manages its resources.

## What you need before you start

Go through this list first. If you cannot tick a box, ask your STACKIT
administrator.

- [ ] **A STACKIT organization** and its **organization ID** (a long ID
      that looks like `a1b2c3d4-...`).
- [ ] **Someone who may create service accounts and give them permissions**
      in that organization. A service account is a login for a program
      instead of a person; costguard needs one.
- [ ] **A Kubernetes cluster** where costguard can run once a week. The
      easiest choice is a STACKIT Kubernetes Engine (SKE) cluster. costguard
      needs very little: a fraction of one CPU for a few minutes a week.
- [ ] **`kubectl`**, the Kubernetes command line tool, installed on your
      computer and connected to that cluster.
      ([How to install kubectl](https://kubernetes.io/docs/tasks/tools/))
- [ ] **A chat channel** in Microsoft Teams, Slack or Google Chat, and the
      permission to add a webhook to it (a webhook is a web address that
      lets a program post into the channel).
- [ ] **About an hour** for the first setup.

## Setup, step by step

This sets up **stage 1**. Stage 1 only reports; it never changes anything in
your cloud, so it is safe to try.

### Step 1: Create the chat webhook

Create a webhook in the channel where the reports should appear, and copy
its web address. You will need it in step 6.

- **Microsoft Teams:** in the channel, open _Workflows_ and use the template
  _"Post to a channel when a webhook request is received"_. Copy the URL it
  shows at the end.
- **Slack:** create an _incoming webhook_ for the channel
  ([Slack guide](https://api.slack.com/messaging/webhooks)).
- **Google Chat:** in the space, open _Apps & integrations_, then
  _Webhooks_, and add one.

Treat this web address like a password: anyone who has it can post into
your channel.

### Step 2: Create a service account for costguard

Create a STACKIT service account, for example named `costguard-report`, and
give it permission to **read** your organization: its folders, projects,
resources and cost data. It needs no permission to change anything in stage

1.

If you only want costguard to look at part of your organization, give it
read access to just those folders or projects, and list them in step 5 **by
their ID**. Names only work when costguard may read the whole organization,
because it has to search for them. Two things to know when costguard may
only read part of the organization:

- **`skip` entries must point to folders or projects inside that part.** An
  entry for something elsewhere counts as "matches nothing", and that blocks
  all marking and deleting.
- **A `do-not-delete` on a folder above that part is only honoured if
  costguard may read that folder.**
- **Cost data can usually only be read for the whole organization.**
  Without it, every report says that something could not be read. Set
  `warnEmptyAfterDays: 0` in `config.yaml` to switch the empty-project
  warnings off.

### Step 3: Choose how costguard logs in

Pick **one** of the two options.

**Option A: workload identity (recommended on SKE, no keys to look after).**
Your SKE cluster proves to STACKIT who costguard is, so there is no password
or key to store. It needs a one-time trust setup between the service account
and the cluster, which your STACKIT administrator can do with Terraform. The
example in [`examples/ske-workload-identity`](../../examples/ske-workload-identity)
shows how. For costguard, the trust must be for the subject
`system:serviceaccount:costguard:costguard-stage1`.

**Option B: a key file (works on any cluster, simplest to start with).**
Create a key for the service account in the STACKIT portal and download it.
It is a small JSON file. Keep it secret: it is a password.

### Step 4: Download costguard

Open the [releases page](https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases),
pick the newest release named **costguard vX.Y.Z**, and download these three
files into one folder:

- `config.yaml`: your settings
- `stage1.yaml`: stage 1 (report only)
- `stage2.yaml`: stage 2 (clean up), for later

The stage files run costguard's container image from our registry,
`professional-service.git.onstackit.cloud/professional-service-best-practices/costguard`,
pinned to exactly that release. We publish it on a best-effort basis. If
your cluster can pull from our registry, you are done with this step.
If it cannot, or your company only runs images it built itself,
[build the image yourself](#building-the-image-yourself) and use the stage
files that this produces instead.

### Step 5: Fill in your settings

Open `config.yaml` in a text editor. You must change two lines:

- `organizationId`: your organization ID.
- `output`: `teams`, `slack` or `googlechat`, depending on your chat.

Everything else is optional and explained in the file:

- **`scope`**: which folders or projects to look at. Leave it empty for the
  whole organization.
- **`skip`**: folders or projects costguard should never look at, by name or
  ID. Put your production projects here if you want to be extra careful.
- **`regions`**: costguard only looks at the region `eu01` unless you say
  otherwise. **If your company also uses other STACKIT regions (for example
  `eu02`), list all of them**, for example `regions: [eu01, eu02]`. Resources
  in regions that are not listed are never looked at, never reported and
  never cleaned up.

Then open `stage1.yaml`:

- **Option A (workload identity):** replace
  `REPLACE-WITH-STAGE1-SA@sa.stackit.cloud` with the email address of the
  service account from step 2.
- **Option B (key file):** delete the whole line that starts with
  `workload-identity.stackit.cloud/service-account-email`.

### Step 6: Install

Open a terminal in the folder with the three files and run these commands.
Replace the parts in `<...>`.

```bash
# Create the costguard area in your cluster and store your settings
kubectl apply -f config.yaml

# Store the chat webhook address from step 1
kubectl -n costguard create secret generic costguard-webhook --from-literal=url='<webhook address>'

# Only for option B: store the key file from step 3
kubectl -n costguard create secret generic costguard-sa-key --from-file=key.json=<path to the key file>

# Start stage 1
kubectl apply -f stage1.yaml
```

### Step 7: Try it right away

You do not have to wait until Monday:

```bash
kubectl -n costguard create job --from=cronjob/costguard-report costguard-first-run
```

A report should appear in your chat within a few minutes. If it does not,
see [Questions and problems](#questions-and-problems).

### Reading the report

Each report tells you:

- **how many things could be cleaned up** and how much deleting them would
  save per month and per year,
- the lists per category, each entry with its project and a link to that
  project in the STACKIT portal,
- warnings about empty projects and network areas,
- at the bottom, how many folders and projects were skipped.

**What is listed:**

- **Everything that will be deleted is always listed in full**, including
  everything someone marked with `delete=true`.
- **New finds are limited to 10 per category per run**, the biggest savings
  first, and costguard only marks the ones it lists. If it found more, the
  message says how many are waiting (_"… and 14 more; they will be listed in
  the next runs."_). They come up in the following runs. With a big backlog,
  [run costguard more often](#big-backlog-run-it-more-often).
- Lists that are only for your information (disks with snapshots, empty
  projects and network areas) also show 10 entries and how many more there
  are.

Let stage 1 run for a few weeks. Each Monday, look at the list and **add
`do-not-delete=true` to everything that must stay**. When the list only
contains things you really want gone, you are ready for stage 2.

## Turning on automatic cleanup

In stage 2, costguard marks what it found on Monday and deletes it on
Tuesday. Before you switch:

- [ ] You have read at least a few Monday reports, and everything that must
      stay has `do-not-delete=true` or is in the `skip` list.
- [ ] Your colleagues know the two labels, and they know that the Monday
      message is their chance to object.
- [ ] You created a second service account (for example
      `costguard-cleanup`) that may **read** the organization, **change
      labels** and **delete** servers, disks, IP addresses, snapshots,
      images, network interfaces and security groups. For option A, its
      trust is for the subject
      `system:serviceaccount:costguard:costguard-stage2`.

Then, **on any day from Tuesday to Sunday** (so that a Monday comes before
the first Tuesday):

1. Open `stage2.yaml` and do the same as in step 5, now with the second
   service account. For option B, replace the key secret with the new key:
   ```bash
   kubectl -n costguard delete secret costguard-sa-key
   kubectl -n costguard create secret generic costguard-sa-key --from-file=key.json=<path to the new key file>
   ```
2. Switch:
   ```bash
   kubectl delete -f stage1.yaml
   kubectl apply -f stage2.yaml
   ```

The first Monday in stage 2 lists everything that is already marked with
`delete=true`, plus up to 10 new finds per category.

### Big backlog: run it more often

costguard marks at most 10 new finds per category per run. With the weekly
rhythm, a backlog of 100 unused disks takes ten weeks. To clear it faster,
run costguard every working day:

1. In `stage2.yaml`, change the two `schedule` lines:
   - `costguard-delete`: `"30 7 * * 1-5"` (Monday to Friday at 07:30)
   - `costguard-flag`: `"0 8 * * 1-5"` (Monday to Friday at 08:00)
2. In `config.yaml`, set
   `deleteRunAt: "the next working day at 07:30 (Europe/Berlin)"`.
3. Apply both files again: `kubectl apply -f config.yaml -f stage2.yaml`.

**On each day, the delete job must run before the flag job** (07:30 before
08:00). Every morning it then deletes what was announced the working day
before, about 23½ hours earlier; what is marked on Friday is deleted on
Monday. If the delete job ran after the flag job on the same day (for
example flag at 08:00, delete at 09:00), things marked at 08:00 would be
deleted an hour later, without a day's notice. Whatever rhythm you choose,
always leave about a day between marking and deleting.

In stage 1 you can report daily the same way: set the `schedule` of
`costguard-report` in `stage1.yaml` to `"0 8 * * 1-5"`.

When the backlog is gone, you can switch back to the weekly schedules
(`"0 8 * * 1"` for the flag job, `"0 8 * * 2"` for the delete job, and
`deleteRunAt: "Tuesday 08:00 (Europe/Berlin)"`).

### Stop it immediately

To stop all deleting right away (for example if a message looks wrong):

```bash
kubectl -n costguard patch cronjob costguard-delete -p '{"spec":{"suspend":true}}'
```

To allow deleting again, run the same command with `false` instead of
`true`.

### Go back to report only

```bash
kubectl delete -f stage2.yaml
kubectl apply -f stage1.yaml
```

The `delete=true` labels stay on the resources. If you switch to stage 2
again later, do it between Tuesday and Sunday again.

### Remove costguard completely

```bash
kubectl delete namespace costguard
```

## How costguard keeps you safe

- **Stage 1 never changes anything.** Its service account cannot even do
  so.
- **Everybody gets a day's notice.** Things costguard finds itself are
  announced on Monday and deleted on Tuesday at the earliest.
- **`do-not-delete=true` always wins,** also on whole folders and projects.
- **A forgotten `delete=true` on a protected resource is cleaned up.** If a
  disk or IP address carries both labels, the Monday run removes the old
  `delete=true`, so lifting the protection later cannot delete it by
  surprise. Other resources with both labels are listed, and the message
  asks you to remove the `delete` label by hand.
- **Nothing is deleted that was not in a message.** Everything already
  marked with `delete=true` is always listed in full, and costguard only
  marks the new finds it lists. The one exception: a `delete=true` someone
  adds after the Monday message (see below).
- **Everything is checked again right before it is deleted.** If someone
  added `do-not-delete=true`, removed `delete=true` or started using the
  resource again in the meantime, it stays.
- **Disks and IP addresses in use are never deleted,** even when marked. If
  one is used again, costguard removes its mark.
- **It only deletes the marked item itself.** Deleting a server does not
  delete its disks; they go through the normal Monday/Tuesday cycle
  afterwards.
- **It never deletes projects, folders or network areas**, and never deletes
  an IP address that a load balancer uses.
- **It never marks disks that have snapshots, or disks in projects that run
  Kubernetes (SKE)** by itself. They are only deleted if someone marks them
  with `delete=true` on purpose.
- **When in doubt, it does nothing.** If costguard cannot read something, it
  leaves it alone and says so in the message.
- **No message, no marks.** If the Monday message cannot be delivered
  (costguard tries three times when the chat service has a hiccup), nothing
  new is marked.
- **Corrections go to the chat too.** If costguard cannot write a label
  after the Monday message went out, it posts a correction the same morning
  that lists what will _not_ be deleted this week.
- **A broken skip list stops everything.** If an entry in `skip` matches
  nothing (for example because a project was renamed), costguard marks and
  deletes nothing until you fix it, and tells you in the message.

### Please be aware

- **`delete=true` means "delete", no matter who set it or when.** If someone
  adds it after the Monday message, the item is deleted on Tuesday without
  having been listed. Only add it when you mean it.
- **If another tool in your company already uses a label called `delete`,**
  costguard treats it the same way. Check before you switch to stage 2.
- **Anyone who can change labels on a resource can have costguard delete
  it.** Keep permissions tidy.
- **There is no limit on how much one Tuesday deletes.** Read the Monday
  message, especially the first one.
- **Before you lift a skip** (remove a folder or project from `skip`, or
  its `do-not-delete`), check it for old `delete=true` labels. costguard
  does not look inside skipped folders and projects, so it cannot clean
  those labels up; after the skip is lifted, such resources are listed on
  the next Monday and deleted on the Tuesday after.
- **If a great many resources are marked at once** (for example someone
  marks 300 servers), the Monday message can get too big for the chat and
  not arrive. They are still deleted on Tuesday. Mark large amounts in
  batches.
- **Keep about a day between the two schedules.** The time between marking
  and deleting comes from the schedules in `stage2.yaml`. If you change them,
  follow [Big backlog: run it more often](#big-backlog-run-it-more-often)
  and update `deleteRunAt` in `config.yaml`.

## Settings

All settings live in `config.yaml`. A typo in a setting name is reported as
an error instead of being silently ignored.

| Setting                                                  | Default                         | What it does                                                                                                                                                                                                                                                   |
| -------------------------------------------------------- | ------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `organizationId`                                         | – (required)                    | Your STACKIT organization.                                                                                                                                                                                                                                     |
| `output`                                                 | – (required)                    | `teams`, `slack` or `googlechat`.                                                                                                                                                                                                                              |
| `scope.folders`, `scope.projects`                        | empty: the whole organization   | Only look at these folders and projects. Names or IDs. Each entry must match exactly one folder or project, otherwise costguard stops and tells you why.                                                                                                       |
| `skip.folders`, `skip.projects`                          | empty                           | Never look at these. Names (upper or lower case does not matter) or IDs. A name used by several folders or projects skips all of them. Entries must point to something inside the `scope`; an entry that matches nothing there stops all marking and deleting. |
| `regions`                                                | `eu01`                          | Which STACKIT regions to look at. **List every region your company uses**, for example `[eu01, eu02]`; other regions are ignored.                                                                                                                              |
| `deleteRunAt`                                            | `Tuesday 08:00 (Europe/Berlin)` | The deletion time shown in the Monday message. Keep it in line with the schedule in `stage2.yaml`.                                                                                                                                                             |
| `warnEmptyAfterDays`                                     | `30`                            | How old an empty project or network area must be before costguard warns about it. **`0` switches these warnings off**; do that if costguard may not read your organization's cost data, otherwise every report says something could not be read.               |
| `prices.publicIpMonthlyEur`, `prices.volumeGbMonthlyEur` | `4.82`, `0.0619`                | Prices used for the savings estimate.                                                                                                                                                                                                                          |
| `portalUrl`                                              | `https://portal.stackit.cloud`  | Where the links in the messages point to.                                                                                                                                                                                                                      |

After changing `config.yaml`, apply it again with
`kubectl apply -f config.yaml`. The next run uses the new settings.

## Questions and problems

**No message arrived.** Look at the log of the last run:

```bash
kubectl -n costguard get jobs
kubectl -n costguard logs job/<name of the job>
```

When something is wrong, costguard posts a message titled _"costguard …
run failed"_ that says what, including mistakes in `config.yaml` (it lists
every problem it found). If **no message at all** arrives, the problem is
in the chat settings themselves (a wrong or non-https webhook address from
step 1, or a wrong `output`), or the run did not start. The log shows
which.

**The message says "costguard cannot log in to STACKIT".** The login from
step 3 is not set up: for option A, the service account annotation is
missing in `stage1.yaml` / `stage2.yaml`; for option B, the
`costguard-sa-key` secret is missing or not a valid key file. The details
under the message say which.

**The message says "costguard cannot read the organization".** costguard
logged in, but may not read your organization. Usually the service account
lacks the read permission from step 2, the annotation names the wrong
service account, or the workload identity trust from step 3 is missing.

**The message says something "could not be read".** The service account is
missing a permission for part of your organization. costguard leaves those
parts alone until it can read them. Check the permissions from step 2.

**The message says deletions are blocked.** An entry in the `skip` list
matches nothing inside the scope. Usually a folder or project was renamed or
deleted, the entry points to something outside your `scope`, or costguard
may not read the folder it is in. Fix or remove the entry in `config.yaml`
and apply it again.

**The run failed because a scope entry matches nothing or several things.**
Use the exact name, or better the ID, of the folder or project in `scope`.

**Something was listed that we still need.** Add `do-not-delete=true` to it
before Tuesday 08:00. If it is marked with `delete=true`, you can also just
remove that label.

**Something was deleted that we still needed.** Deleted resources cannot be
restored by costguard. Check the Tuesday message and the log to see why it
was deleted (it was marked with `delete=true` and had no
`do-not-delete=true`), and protect similar resources with
`do-not-delete=true`.

**Does costguard delete projects or whole folders?** No. It only warns about
empty projects and network areas.

**What does costguard itself cost?** Almost nothing: one small program that
runs for a few minutes once or twice a week.

## For developers

```bash
make test    # unit tests with a per-package 80% coverage gate
make lint    # go vet
go run ./cmd/costguard --config my-config.yaml report   # report only, changes nothing
ko build -B --local -t dev ./cmd/costguard               # container image into the local Docker daemon
```

The tests never call STACKIT: `internal/fake` is an in-memory STACKIT. Every
merge to `main` that changes costguard is released automatically by
`.github/workflows/costguard-release.yaml` on our Forgejo instance: the
version comes from the commit messages (`fix:` → patch, `feat:` → minor),
the image goes to
`professional-service.git.onstackit.cloud/professional-service-best-practices/costguard`,
and the release carries `config.yaml`, `stage1.yaml` and `stage2.yaml` with
the image pinned. Pull requests build no image; to get one, add the label
`build-image` to the pull request, and the workflow pushes a test image
tagged `pr-<number>`. What changed in each version is on the
[releases page](https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases).

### Building the image yourself

You need Git, Go (the version in [`go.mod`](go.mod)), [ko](https://ko.build)
and a container registry that your cluster can pull from, for example a
STACKIT Container Registry project. Run this on your machine or in your own
pipeline, with the version of the release you downloaded:

```bash
git clone https://github.com/stackitcloud/professional-service.git
cd professional-service
git checkout apps/costguard/vX.Y.Z
cd apps/costguard

ko login <your registry> --username <user> --password-stdin <<<'<password>'
export KO_DOCKER_REPO=<your registry>/<path>
export COSTGUARD_VERSION=vX.Y.Z

# Builds the image, pushes it and writes stage files pinned to it
ko resolve -B -f deploy/stage1.yaml > stage1.yaml
ko resolve -B -f deploy/stage2.yaml > stage2.yaml
```

Use these two stage files instead of the ones from the release, together
with the release's `config.yaml`. If your cluster needs a login for your
registry, add an `imagePullSecrets` entry to the two service accounts in
the stage files.

---

_costguard is maintained as part of the [STACKIT professional-service
best-practice library](https://github.com/stackitcloud/professional-service)._
