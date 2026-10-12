# Getting Started

Step-by-step guide to pick this project up and get `whisper-fn` running --
locally first, then deployed to STACKIT. This is the as-built path; see
`DECISIONS.md` for why it diverges from the original plan (short version:
`large-v3` does not fit any STACKIT Functions plan, and the default
buildpack build can't be pushed to this project's Container Registry).

## Current live deployment

- URL: `https://<function-id>.functions.onstackit.cloud`
- Project: `functions` (`<your-stackit-project-id>`)
- Model: `small` (not `large-v3` -- see `DECISIONS.md`)
- Plan: `f6` (4096 MB), concurrency 1
- Image: `registry.onstackit.cloud/<your-registry-project>/whisper-fn:0.1.0`

## Prerequisites

- Docker installed and running (`docker info` should succeed)
- `sfn` CLI >= 1.7.0 (`./install-sfn.sh`; older versions fail auth with
  "outdated cli" against this project)
- `curl`
- A STACKIT project with Functions enabled, `Object Storage bucket`, and
  `Container Registry` already set up (see below -- all three already exist
  for this project as of this writing)

## 1. Install/update the STACKIT Functions CLI (`sfn`)

```bash
cd apps/whisper-fn   # this directory
./install-sfn.sh
export PATH="$HOME/.local/bin:$PATH"   # add to ~/.zshrc to persist
sfn --version   # must be >= 1.7.0
```

## 2. Build the function container (Dockerfile, not buildpacks)

```bash
cd whisper-fn
docker build --platform linux/amd64 -t whisper-fn:local .
```

This is a hand-written `Dockerfile`, deliberately **not**
`sfn functions build` (Cloud Native Buildpacks). The buildpack output
includes a zero-byte metadata layer that this project's STACKIT Harbor
registry rejects on push (`blob unknown to registry`) -- see
`DECISIONS.md`. The Dockerfile bakes the `small` model weights in at build
time so cold starts don't re-download them, creates the required non-root
`uid 1001` user, and serves via `uvicorn asgi:app` (see `asgi.py` for the
hand-rolled ASGI lifespan wrapper that the buildpack path would normally
generate for you).

`--platform linux/amd64` is required even on Apple Silicon -- STACKIT
Functions' Knative runtime only runs `linux/amd64` images.

## 3. Run it locally and smoke-test

```bash
docker run -d --platform linux/amd64 -p 8080:8080 -e PORT=8080 \
  --env-file .env --name whisper-fn-local whisper-fn:local
curl http://127.0.0.1:8080/health
# {"status": "ok", "model": "small"}
curl -X POST http://127.0.0.1:8080/transcribe
# downloads elephants_dream.mp4 from the bucket, transcribes, uploads the
# transcript back -- takes 90-150s on CPU
docker rm -f whisper-fn-local
```

`whisper-fn/.env` already has working credentials for the `your-bucket-name`
bucket, which already contains `elephants_dream.mp4`. If you need to
recreate it elsewhere: see `.env.example` for the fields, and `stackit
object-storage bucket create` / `credentials create` to provision a new one.

## 4. (Optional, no Docker needed) Run the pure logic test

```bash
cd whisper-fn
python3 test_handler_local.py
```

Stubs out `boto3`/`whisper`/`imageio_ffmpeg` in memory; checks routing and
the S3 upload flow without real transcription or a real S3 connection.

## 5. Push to the STACKIT Container Registry

The registry itself has no CLI or Terraform support -- it's created once
via the STACKIT Portal (Container Registry -> New Project -> Robot Account
with Pull+Push permissions for CI use). This project already has one:
registry project `<your-registry-project>`.

```bash
docker login registry.onstackit.cloud \
  --username 'robot$<your-registry-project>+<robot-name>'
  # password: the robot account's secret from the Portal

docker tag whisper-fn:local registry.onstackit.cloud/<your-registry-project>/whisper-fn:<version>
docker push registry.onstackit.cloud/<your-registry-project>/whisper-fn:<version>
```

**The tag must be SemVer** (e.g. `0.1.0`, optionally with a `v` prefix), or
`latest` combined with an explicit digest. Plain tags like `small` or
`local` are rejected by `sfn functions deploy` with `oci image reference
invalid`.

If `docker login` fails with `error storing credentials ... User
interaction is not allowed` (macOS Keychain inaccessible, e.g. from a
non-interactive shell), write the base64 auth directly into an isolated
`DOCKER_CONFIG` directory instead of your real `~/.docker/config.json`:

```bash
mkdir -p /tmp/docker-config
python3 -c "
import base64, json
auth = base64.b64encode(b'robot\$<your-registry-project>+<robot-name>:<secret>').decode()
json.dump({'auths': {'registry.onstackit.cloud': {'auth': auth}}}, open('/tmp/docker-config/config.json', 'w'))
"
docker --config /tmp/docker-config push registry.onstackit.cloud/<your-registry-project>/whisper-fn:<version>
```

## 6. Register the runtime pull-secret (once per project)

```bash
sfn pull-credentials create --address registry.onstackit.cloud \
  --pull-credential-name whisper-fn-registry
# prompts for username (robot$...) and password interactively --
# --no-interactive is explicitly disallowed here for security reasons
```

## 7. Update `.stackit-functions/revision.yaml`

Set `spec.image` to the pushed `<registry>/<project>/<image>:<version>`
reference, and `spec.limits.plan` to a plan with enough memory. For
`small`, empirically: `f3` (512 MB) and `f4` (1024 MB) OOM-kill; `f5`
(2048 MB) and `f6` (4096 MB) both work. This project runs `f6`. Test your
own plan choice locally first:

```bash
docker run -d --platform linux/amd64 --memory=<plan-mb>m --memory-swap=<plan-mb>m \
  -p 8080:8080 -e PORT=8080 --env-file .env --name whisper-fn-memtest whisper-fn:local
# wait, then:
docker inspect -f 'OOMKilled={{.State.OOMKilled}}' whisper-fn-memtest
docker rm -f whisper-fn-memtest
```

## 8. Log in and deploy

```bash
sfn auth login --project-id <project-id> \
  --service-account-key-path ~/.stackit/sa-key.json
  # or with a user account via browser login

cd whisper-fn
sfn functions deploy --project-id <project-id> --env-file .env --no-interactive -v
```

No `--build`/`--push` flags needed -- omitting them means "don't rebuild,
don't repush," since the image is already in the registry from step 5.
(Note: these are boolean switches, not `key=value` options --
`--build=false` is a CLI parse error, just omit the flag entirely.)

## 9. Verify

```bash
sfn function describe --function-id <id>   # shows the public URL
curl https://<function-url>/health
curl -X POST https://<function-url>/transcribe
```

Then check the bucket for the uploaded transcript.

## Known constraints

- **No model bigger than `small` fits any STACKIT Functions plan.** Plans
  cap at `f6` / 4096 MB with 1 shared vCPU; both `large-v3` and `medium`
  were OOM-killed even at that ceiling. See `DECISIONS.md`.
- **No documented request-timeout limit** on STACKIT Functions, so the
  90-150s transcription time for `small` is not a gateway-timeout risk.
- Concurrency is set to 1: each `/transcribe` call is a single CPU-bound
  job that would contend for the same memory/CPU budget if run in parallel.

## If something's unclear

`docs/history/Task.md` has the original design rationale and open questions;
`docs/history/Handout.md` has the session-by-session implementation log;
`DECISIONS.md` has the non-trivial choices made and why.
