# Custom-build image for whisper-fn, used instead of the default Cloud
# Native Buildpacks path because this project's STACKIT Harbor registry
# rejects the zero-byte metadata layer CNB adds to every image
# ("blob unknown to registry" on push) -- a plain Dockerfile build doesn't
# produce that layer, so pushing actually succeeds.
#
# Build (from this directory, whisper-fn/):
#   docker build --platform linux/amd64 -t whisper-fn:small .
#
# Deploy (after pushing to a registry -- see ../GETTING-STARTED.md):
#   sfn function deploy --build=false --push=true --image <registry>/whisper-fn:small
#
# STACKIT Functions' container interface requires: HTTP server on $PORT,
# SIGTERM handling, non-root numeric user. See
# docs.functions.onstackit.cloud/how-tos/create-custom-functions.html.

FROM --platform=linux/amd64 python:3.11-slim

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    WHISPER_MODEL=small \
    HOME=/home/appuser

# Create the non-root user STACKIT Functions requires before anything else,
# so its home dir (and later the baked-in model cache under it) is owned by
# uid 1001 from the start.
RUN useradd --create-home --uid 1001 --shell /usr/sbin/nologin appuser

WORKDIR /app
COPY pyproject.toml handler.py asgi.py ./

# CPU-only PyTorch wheel (much smaller than the default CUDA build); must
# land before "pip install ." so the whisper dependency resolves against it
# instead of pulling the CUDA build from PyPI.
RUN pip install --no-cache-dir torch --index-url https://download.pytorch.org/whl/cpu
RUN pip install --no-cache-dir .

# handler.py symlinks imageio_ffmpeg's static binary to "ffmpeg" inside its
# own site-packages dir the first time it's imported -- do that now, as
# root, while site-packages is still writable. The symlink check is
# idempotent (`if not os.path.exists`), so importing handler.py again at
# runtime as uid 1001 just finds it already there instead of failing with
# PermissionError.
RUN python -c "import handler"

# Running the two pip installs as root under linux/amd64 emulation (e.g.
# Rosetta on Apple Silicon) can leave root-owned files under $HOME/.cache
# (e.g. .cache/rosetta), which would block appuser from creating
# .cache/whisper in the next step -- reclaim ownership before switching.
RUN chown -R appuser:appuser /home/appuser

USER 1001

# Bake the small weights into the image as uid 1001, so handler.py's
# whisper.load_model(MODEL_SIZE) (no download_root override) finds them
# already cached under $HOME/.cache/whisper at cold start instead of
# fetching them from openaipublic.azureedge.net on every scale-up.
RUN python -c "import whisper; whisper.load_model('small')"

EXPOSE 8080
CMD uvicorn asgi:app --host 0.0.0.0 --port ${PORT:-8080}
