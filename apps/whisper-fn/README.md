<!-- tags: functions, serverless, object-storage, s3, ai-ml, scale-to-zero -->

# whisper-fn

Rebuilds an always-on VM transcription demo as a **STACKIT Function**: a
container that only runs while it's handling a request and scales to zero
afterwards, instead of a VM that runs (and costs) 24/7.

## What it does

An HTTP-triggered function (`whisper-fn/`) that, on `POST /transcribe`:

1. downloads a video from an S3 (STACKIT Object Storage) bucket,
2. transcribes it locally with [`openai-whisper`](https://github.com/openai/whisper)
   (the open-weights Whisper model running inside the function — **not** a call
   to OpenAI's paid API),
3. uploads the resulting transcript back to the same bucket,
4. returns a small JSON summary (bucket, keys, model, duration).

`GET /health` returns a plain status check.

## Why it exists

A Terraform-provisioned-VM version of this demo works, but it runs continuously
via `cloud-init` and only ever leaves its result on local disk — nobody stops
it, so it costs money the whole time it's not transcribing anything. STACKIT
Functions offered a scale-to-zero, HTTP-triggered alternative, so this project
reimplements the same demo (same input video, same Whisper model family) as a
function instead of a VM, with the transcript written back to S3 so the result
is actually retrievable afterwards.

This project provisions its own S3 bucket and Container Registry rather than
assuming you already have one set up — see [GETTING-STARTED.md](GETTING-STARTED.md)
for the one-time manual steps.

## Project layout

| Path | Purpose |
|---|---|
| `whisper-fn/` | The actual function: `handler.py`, dependencies, STACKIT Functions manifests |
| `whisper-fn/.env.example` | Template for the env vars you need (S3 bucket/credentials, model size) — copy to `whisper-fn/.env`, fill in your own values, never commit it |
| `install-sfn.sh` | Installs the `sfn` CLI (STACKIT Functions CLI) locally |
| `setup.sh` | One-command local bootstrap: installs the sfn CLI, prepares `whisper-fn/.env`. Creates no cloud resources. |
| `deploy.sh` | Convenience wrapper: build → auth → confirm → deploy, using `whisper-fn/.env` |
| `GETTING-STARTED.md` | Step-by-step setup and deploy instructions |
| `docs/history/Task.md` | Original design/planning notes (background reading, not required to run this) |
| `docs/history/Handout.md` | Session log — implementation history and the disk-space blocker that paused the first deploy attempt (background reading, not required to run this) |

## Status

Verified end-to-end with a real transcription of `elephants_dream.mp4`
(downloaded from the bucket, transcribed, transcript written back), running
`WHISPER_MODEL=small`, not the originally planned `large-v3` — see
`DECISIONS.md` for why (short version: no STACKIT Functions plan has enough
memory for `large-v3` or even `medium`). Built via a hand-written
`Dockerfile`, not the default buildpack path — see `DECISIONS.md` for that
too (buildpack images get rejected by this registry).

## Quick start

```bash
cd apps/whisper-fn
./setup.sh                                         # installs sfn CLI, prepares whisper-fn/.env (no cloud resources created)
```

Then, after filling in `whisper-fn/.env` with your own S3 bucket/credentials:

```bash
./deploy.sh <project-id> <registry-image-ref> <registry-username>
# e.g. ./deploy.sh <your-stackit-project-id> registry.onstackit.cloud/<reg-project>/whisper-fn:0.1.1 'robot$<reg-project>+<robot-name>'
```

See [`GETTING-STARTED.md`](GETTING-STARTED.md) for the full walkthrough --
creating the Object Storage bucket and Container Registry are one-time,
manual (Portal) steps not covered by `setup.sh` or `deploy.sh`.

## Cost note

This project creates real STACKIT resources once deployed (S3 bucket, a running
function instance while it's warm). Check your STACKIT budget/alerting before
deploying, and tear the function down (`sfn function delete`) when you're done
with it.
