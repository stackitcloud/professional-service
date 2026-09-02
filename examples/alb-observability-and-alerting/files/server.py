#!/usr/bin/env python3
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

# Minimal HTTP backend whose health, status code, delay and response size
# can be chosen per request, so that alert rules can be triggered on purpose.
# Usage: server.py [port]
import json
import socket
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
HOSTNAME = socket.gethostname()
FAIL_SECONDS = 300
STATE = {"unhealthy_until": 0.0}
LOCK = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):
        # Silence the per-request log of BaseHTTPRequestHandler; the health
        # checks of the load balancer alone would add a line every few seconds.
        pass

    def reply(self, status, body):
        self.send_response(status)
        self.send_header("X-Backend", HOSTNAME)
        if status in (204, 304):
            # these status codes must not carry a body
            self.end_headers()
            return
        body["backend"] = HOSTNAME
        payload = (json.dumps(body) + "\n").encode()
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(payload)

    def do_GET(self):
        path = self.path.split("?", 1)[0].rstrip("/") or "/"
        parts = path.strip("/").split("/")
        if path == "/":
            return self.reply(200, {"message": "hello"})
        if path == "/healthz":
            with LOCK:
                remaining = max(0, int(STATE["unhealthy_until"] - time.time()))
            healthy = remaining == 0
            return self.reply(
                200 if healthy else 503,
                {"healthy": healthy, "unhealthy_for_s": remaining},
            )
        if parts[0] == "healthz" and len(parts) >= 2 and parts[1] == "fail":
            # unhealthy for FAIL_SECONDS, or for /healthz/fail/<seconds>
            if len(parts) == 3 and parts[2].isdecimal():
                seconds = min(int(parts[2]), 3600)
            elif len(parts) == 2:
                seconds = FAIL_SECONDS
            else:
                return self.reply(404, {"error": "not found"})
            with LOCK:
                STATE["unhealthy_until"] = time.time() + seconds
            return self.reply(200, {"healthy": False, "unhealthy_for_s": seconds})
        if path == "/healthz/ok":
            with LOCK:
                STATE["unhealthy_until"] = 0.0
            return self.reply(200, {"healthy": True, "unhealthy_for_s": 0})
        if parts[0] == "status" and len(parts) == 2 and parts[1].isdecimal():
            status = int(parts[1])
            if 200 <= status <= 599:
                return self.reply(status, {"status": status})
        if parts[0] == "delay" and len(parts) == 2 and parts[1].isdecimal():
            delay_ms = min(int(parts[1]), 30000)
            time.sleep(delay_ms / 1000)
            return self.reply(200, {"delay_ms": delay_ms})
        if parts[0] == "bytes" and len(parts) == 2 and parts[1].isdecimal():
            size = min(int(parts[1]), 1048576)
            return self.reply(200, {"bytes": size, "payload": "x" * size})
        return self.reply(404, {"error": "not found"})

    do_HEAD = do_GET


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
