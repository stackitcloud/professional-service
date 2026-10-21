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

"""ASGI entrypoint for the custom-build (Dockerfile) deployment path.

The buildpack path's generated entrypoint normally wraps handler.py's
Function.start()/stop()/handle() in an ASGI app with lifespan support --
see docs.functions.onstackit.cloud/reference/generated-code.html. Since a
Dockerfile-built image skips that generated entrypoint, this file recreates
the same lifespan wiring so `uvicorn asgi:app` boots and shuts down
correctly (Function.start() loads the Whisper model once per warm
instance; Function.stop() runs on SIGTERM via uvicorn's graceful shutdown).

Used instead of the buildpack build because the project's STACKIT Harbor
registry rejects the zero-byte metadata layer Cloud Native Buildpacks adds
("blob unknown to registry") -- a plain Dockerfile build doesn't produce
that layer.
"""

from handler import new

_fn = new()


async def app(scope, receive, send):
    if scope["type"] == "lifespan":
        while True:
            message = await receive()
            if message["type"] == "lifespan.startup":
                await _fn.start()
                await send({"type": "lifespan.startup.complete"})
            elif message["type"] == "lifespan.shutdown":
                await _fn.stop()
                await send({"type": "lifespan.shutdown.complete"})
                return
        return

    await _fn.handle(scope, receive, send)
