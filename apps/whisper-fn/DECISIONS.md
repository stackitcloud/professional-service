# Decisions

## 2026-10-02: WHISPER_MODEL small, not large-v3

**Was:** Deploy whisper-fn with `WHISPER_MODEL=small`, not `large-v3` as originally requested.

**Warum:** STACKIT Functions plans cap at `f6` (4096 MB RAM, 1 shared vCPU) --
there is no bigger self-service tier; a larger plan requires contacting
STACKIT directly, with no guaranteed turnaround. Empirically tested all
three model sizes under memory caps matching the real plans:

- `large-v3` (2.88 GB checkpoint): OOM-killed at 4096 MB (`f6`, the max plan),
  and even failed to *load* during a local Docker build at 7.75 GB (needed
  11.67 GB to succeed there).
- `medium` (1.42 GB checkpoint): also OOM-killed at 4096 MB.
- `small` (~500 MB checkpoint): confirmed working end-to-end (real
  transcription of `elephants_dream.mp4`) under caps down to 2048 MB.

**Alternative verworfen:** Hosting `large-v3` on an always-on STACKIT Server/VM
instead of Functions would work memory-wise, but loses Scale-to-Zero and runs
(and bills) continuously.
Not pursued since the user confirmed `small` is acceptable.

## 2026-10-02: Dockerfile custom-build, not Cloud Native Buildpacks

**Was:** Build `whisper-fn`'s image via a hand-written `Dockerfile`
(`sfn function deploy --build=false --push=false`), not the default
`sfn functions build` (buildpacks) path.

**Warum:** The buildpack-built image includes a zero-byte metadata layer
that this project's STACKIT Harbor registry rejects on push
(`error from registry: blob unknown to registry -
sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` --
the well-known empty-blob digest). Reproduced consistently across retries;
all other layers pushed fine. A plain Dockerfile build (ordinary
`RUN`/`COPY` layers) does not produce that layer, and pushed successfully.

This was initially built for `large-v3` (to bake the model weights into
the image at build time, since buildpacks have no `apt-get` and openai-whisper
needs `ffmpeg`), then rebuilt for `small` after the model-size decision
above. The `small` Dockerfile dropped the `large-v3`-specific RAM tuning
(Docker Desktop memory bump) since `small` loads fine without it.

**Alternative verworfen:** Pushing to a different registry (ghcr.io, Docker
Hub) instead of fixing the buildpack image -- would have avoided the bug
but moves the image outside STACKIT's own registry; not pursued per user's
choice to stay on the official STACKIT Container Registry path.

## 2026-10-02: Plan f6, concurrency 1

**Was:** Deployed revision uses `plan: f6` (4096 MB) and `concurrency: 1`,
not the template defaults (`f3` / 512 MB, concurrency 50).

**Warum:** `small` was empirically confirmed to need more than the `f3`
(512 MB) and `f4` (1024 MB) tiers -- both OOM-killed. `f5` (2048 MB) passed
a full end-to-end transcription test locally and was initially deployed;
user then asked for `f6` for extra headroom, so that's the live
configuration. Concurrency was dropped from the template's default of 50 to
1, since each `/transcribe` request is a single CPU-bound ~100-150s job on
a 1-shared-vCPU plan -- running several in parallel would contend for the
same CPU/memory budget that's already near its ceiling.

## 2026-10-02: Image tag must be SemVer

**Was:** Pushed image as `whisper-fn:0.1.0`, not `whisper-fn:small`.

**Warum:** `sfn functions deploy` rejected the `:small` tag with
`Error: Error creating function revision: oci image reference invalid` --
STACKIT Functions requires either a SemVer tag (with or without `v` prefix)
or `latest` combined with a digest. Not documented in the Functions docs
mirror at the time of this session; discovered via the deploy error message
itself.
