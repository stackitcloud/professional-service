# Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
Lightweight local smoke test for handler.py that needs NO Docker, NO Buildpacks,
and NO heavy pip installs (torch/whisper/boto3 are stubbed via sys.modules).

Why this exists: it lets you verify the ASGI routing/response logic in
handler.py is correct without Docker, heavy pip installs, or real S3/STACKIT
credentials -- useful any time those aren't available or convenient (it was
originally written to work around a disk-space outage blocking the local
Docker daemon; see docs/history/Handout.md for that history). It does NOT
exercise real Whisper transcription or real S3 calls -- those still require
the full container build (see GETTING-STARTED.md).

Run: python3 test_handler_local.py
"""
import asyncio
import sys
import tempfile
import types


def _install_stub_modules():
    # --- stub boto3 ---
    boto3_stub = types.ModuleType("boto3")

    class _FakeS3Client:
        def download_file(self, bucket, key, path):
            with open(path, "wb") as f:
                f.write(b"fake-video-bytes")

        def upload_file(self, path, bucket, key):
            with open(path, "r", encoding="utf-8") as f:
                _FakeS3Client.last_uploaded_text = f.read()

    def _client(service, **kwargs):
        assert service == "s3"
        return _FakeS3Client()

    boto3_stub.client = _client
    sys.modules["boto3"] = boto3_stub

    # --- stub imageio_ffmpeg ---
    # handler.py symlinks this path to a sibling "ffmpeg" file at import time,
    # so the fake path needs to (a) actually exist and (b) not already be
    # named "ffmpeg", mirroring imageio_ffmpeg's real versioned filenames.
    fake_ffmpeg_dir = tempfile.mkdtemp(prefix="fake-ffmpeg-")
    fake_ffmpeg_exe = f"{fake_ffmpeg_dir}/ffmpeg-fake-v0"
    open(fake_ffmpeg_exe, "w").close()
    ffmpeg_stub = types.ModuleType("imageio_ffmpeg")
    ffmpeg_stub.get_ffmpeg_exe = lambda: fake_ffmpeg_exe
    sys.modules["imageio_ffmpeg"] = ffmpeg_stub

    # --- stub whisper ---
    whisper_stub = types.ModuleType("whisper")

    class _FakeModel:
        def transcribe(self, path):
            return {"text": "hello from fake whisper"}

    whisper_stub.load_model = lambda size: _FakeModel()
    sys.modules["whisper"] = whisper_stub

    return _FakeS3Client


async def _call(fn, scope):
    sent = []

    async def send(message):
        sent.append(message)

    async def receive():
        return {"type": "http.disconnect"}

    await fn.handle(scope, receive, send)
    status = sent[0]["status"]
    body = sent[1]["body"].decode("utf-8")
    return status, body


async def main():
    fake_s3_cls = _install_stub_modules()
    import importlib

    handler = importlib.import_module("handler")

    fn = handler.new()
    await fn.start()

    # GET /health -> 200
    status, body = await _call(fn, {"type": "http", "method": "GET", "path": "/health"})
    assert status == 200, f"expected 200, got {status}: {body}"
    print(f"[PASS] GET /health -> {status} {body}")

    # GET /unknown -> 404
    status, body = await _call(fn, {"type": "http", "method": "GET", "path": "/unknown"})
    assert status == 404, f"expected 404, got {status}: {body}"
    print(f"[PASS] GET /unknown -> {status} {body}")

    # POST /transcribe without S3_BUCKET set -> 500 with a clear error message
    status, body = await _call(fn, {"type": "http", "method": "POST", "path": "/transcribe"})
    assert status == 500, f"expected 500, got {status}: {body}"
    assert "S3_BUCKET" in body
    print(f"[PASS] POST /transcribe (no S3_BUCKET) -> {status} {body}")

    # POST /transcribe with S3_BUCKET set -> full fake pipeline runs end-to-end
    # NOTE: handler.py reads S3_BUCKET as a module-level constant at import time
    # (correct for real deployments, where env vars are fixed at container start).
    # Setting os.environ here wouldn't retroactively change it, so we patch the
    # already-imported module's constant directly instead.
    handler.S3_BUCKET = "fake-bucket"
    status, body = await _call(fn, {"type": "http", "method": "POST", "path": "/transcribe"})
    assert status == 200, f"expected 200, got {status}: {body}"
    assert "fake-bucket" in body
    print(f"[PASS] POST /transcribe (fake S3+whisper) -> {status} {body}")
    print(f"[PASS] transcript uploaded to fake S3: {fake_s3_cls.last_uploaded_text!r}")

    await fn.stop()
    print("\nAll local routing/logic checks passed (Docker/whisper/S3 stubbed).")


if __name__ == "__main__":
    asyncio.run(main())
