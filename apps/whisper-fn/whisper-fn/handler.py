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

import asyncio
import json
import os
import tempfile
import time

import boto3
import imageio_ffmpeg
import whisper

# openai-whisper shells out to a literal "ffmpeg" binary on PATH. STACKIT
# Functions containers are built via Cloud Native Buildpacks (no apt-get
# access), so we can't install ffmpeg as a system package. imageio-ffmpeg
# ships a static ffmpeg binary via pip instead, but under a versioned
# filename (e.g. "ffmpeg-linux-aarch64-v7.0.2"), not "ffmpeg" -- putting its
# directory on PATH alone isn't enough, so symlink it to the expected name.
_ffmpeg_exe = imageio_ffmpeg.get_ffmpeg_exe()
_ffmpeg_dir = os.path.dirname(_ffmpeg_exe)
_ffmpeg_link = os.path.join(_ffmpeg_dir, "ffmpeg")
if not os.path.exists(_ffmpeg_link):
    os.symlink(_ffmpeg_exe, _ffmpeg_link)
os.environ["PATH"] = _ffmpeg_dir + os.pathsep + os.environ.get("PATH", "")

MODEL_SIZE = os.environ.get("WHISPER_MODEL", "small")
S3_BUCKET = os.environ.get("S3_BUCKET")
S3_INPUT_KEY = os.environ.get("S3_INPUT_KEY", "elephants_dream.mp4")
S3_OUTPUT_KEY = os.environ.get("S3_OUTPUT_KEY", "elephants_dream.txt")
S3_ENDPOINT_URL = os.environ.get("S3_ENDPOINT_URL")  # STACKIT Object Storage endpoint
S3_REGION = os.environ.get("S3_REGION", "eu01")

_model = None  # loaded once per warm instance, see Function.start()


def new():
    return Function()


def _s3_client():
    return boto3.client(
        "s3",
        endpoint_url=S3_ENDPOINT_URL,
        region_name=S3_REGION,
        aws_access_key_id=os.environ.get("S3_ACCESS_KEY"),
        aws_secret_access_key=os.environ.get("S3_SECRET_KEY"),
    )


class Function:
    async def start(self):
        # Load the whisper model once at warm-up instead of per request --
        # model loading dominates cold-start time otherwise.
        global _model
        _model = whisper.load_model(MODEL_SIZE)

    async def stop(self):
        return

    async def handle(self, scope, receive, send):
        assert scope["type"] == "http"
        method = scope.get("method", "GET")
        path = scope.get("path", "/")

        if method == "GET" and path == "/health":
            return await self._respond(send, 200, {"status": "ok", "model": MODEL_SIZE})

        if method == "POST" and path == "/transcribe":
            try:
                result = await self._transcribe()
                return await self._respond(send, 200, result)
            except Exception as exc:  # surface transcription/S3 errors to the caller
                return await self._respond(send, 500, {"error": str(exc)})

        return await self._respond(send, 404, {"error": "not_found"})

    async def _transcribe(self):
        # whisper.transcribe() is CPU-bound and blocking -- run it off the
        # event loop so the ASGI server can still serve /health concurrently.
        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(None, self._transcribe_sync)

    def _transcribe_sync(self):
        if not S3_BUCKET:
            raise RuntimeError("S3_BUCKET env var not set")

        s3 = _s3_client()
        with tempfile.TemporaryDirectory() as tmp:
            video_path = os.path.join(tmp, "input.mp4")
            s3.download_file(S3_BUCKET, S3_INPUT_KEY, video_path)

            model = _model or whisper.load_model(MODEL_SIZE)
            started = time.time()
            result = model.transcribe(video_path)
            duration = time.time() - started

            text_path = os.path.join(tmp, "output.txt")
            with open(text_path, "w", encoding="utf-8") as f:
                f.write(result["text"])

            # Upload the transcript back to S3 -- unlike the old VM version,
            # which only left the result on local disk.
            s3.upload_file(text_path, S3_BUCKET, S3_OUTPUT_KEY)

        return {
            "bucket": S3_BUCKET,
            "input_key": S3_INPUT_KEY,
            "output_key": S3_OUTPUT_KEY,
            "model": MODEL_SIZE,
            "seconds": round(duration, 1),
        }

    @staticmethod
    async def _respond(send, status, payload):
        body = json.dumps(payload).encode("utf-8")
        await send(
            {
                "type": "http.response.start",
                "status": status,
                "headers": [(b"content-type", b"application/json")],
            }
        )
        await send({"type": "http.response.body", "body": body})
