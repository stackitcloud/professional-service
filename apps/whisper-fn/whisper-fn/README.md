# whisper-fn

STACKIT Function: transcribes a video from S3 with `openai-whisper` and writes
the transcript back to S3. See the top-level [`../README.md`](../README.md) for
what this is and why, and [`../GETTING-STARTED.md`](../GETTING-STARTED.md) for
setup/build/deploy steps.

## Project Structure

- `handler.py`: main entry point — routing, S3 download/upload, Whisper call.
- `stackit-functions.yaml` / `.stackit-functions/`: STACKIT Functions manifests
  (created by `sfn functions bootstrap`).
- `test_handler_local.py`: routing/logic smoke test with `boto3`/`whisper`
  stubbed in memory — no Docker or heavy deps needed.

## Routes

- `GET /health` → `200 {"status": "ok", "model": "<WHISPER_MODEL>"}`
- `POST /transcribe` → downloads `S3_INPUT_KEY` from `S3_BUCKET`, transcribes it,
  uploads the result to `S3_OUTPUT_KEY`, returns a JSON summary.
- anything else → `404`

## Environment variables

Copy [`.env.example`](.env.example) to `.env` and fill in your own values
(never commit `.env` — it's gitignored). `../deploy.sh` reads it via
`sfn functions deploy --env-file .env`.

| Variable | Default | Purpose |
|---|---|---|
| `WHISPER_MODEL` | `small` | Whisper model size (kept small deliberately — see timeout risk in `../docs/history/Task.md`) |
| `S3_BUCKET` | *(required)* | Bucket to read the video from / write the transcript to |
| `S3_INPUT_KEY` | `elephants_dream.mp4` | Object key of the source video |
| `S3_OUTPUT_KEY` | `elephants_dream.txt` | Object key the transcript is written to |
| `S3_ENDPOINT_URL` | *(none)* | STACKIT Object Storage endpoint |
| `S3_REGION` | `eu01` | Object Storage region |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | *(none)* | S3 credentials |

## Local Development

Built via a hand-written `Dockerfile`, **not** `sfn functions build`
(Cloud Native Buildpacks) -- the buildpack output includes a zero-byte
metadata layer that this project's STACKIT Harbor registry rejects on
push. See `../DECISIONS.md`.

```bash
docker build --platform linux/amd64 -t whisper-fn:local .
docker run -d --platform linux/amd64 -p 8080:8080 -e PORT=8080 \
  --env-file .env --name whisper-fn-local whisper-fn:local
curl http://localhost:8080/health
docker rm -f whisper-fn-local
```

See `../GETTING-STARTED.md` for the full build/push/deploy walkthrough.

## Function Deployment

See `../GETTING-STARTED.md` steps 5-9 (push to registry, register
pull-secret, update `.stackit-functions/revision.yaml`, then
`sfn functions deploy --project-id <id> --env-file .env`). Not a bare
`sfn function deploy` -- the image is built and pushed manually first.
