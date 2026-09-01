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

# HTTP backend for load balancer routing tests. The same application listens
# on one port per target pool and every response names the pool and the VM
# that served it. Each pool can be marked unhealthy on purpose and the
# application answers WebSocket upgrades with an echo service.
# Usage: server.py <port>=<pool> [<port>=<pool> ...]
import base64
import hashlib
import json
import socket
import struct
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit

# port (as string) -> pool name
APPLICATIONS = dict(arg.split("=", 1) for arg in sys.argv[1:]) or {"8080": "default"}
HOSTNAME = socket.gethostname()
FAIL_SECONDS = 300
WS_GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
LOCK = threading.Lock()
UNHEALTHY_UNTIL = {port: 0.0 for port in APPLICATIONS}


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):
        # Silence the per-request log of BaseHTTPRequestHandler; the health
        # checks of the load balancer alone would add a line every few seconds.
        pass

    @property
    def port(self):
        return str(self.server.server_address[1])

    @property
    def pool(self):
        return APPLICATIONS[self.port]

    def reply(self, status, body):
        body.update({"pool": self.pool, "backend": HOSTNAME, "port": int(self.port)})
        payload = (json.dumps(body) + "\n").encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(payload)

    def do_GET(self):
        url = urlsplit(self.path)
        parts = [part for part in url.path.split("/") if part]
        if "healthz" in parts:
            return self.health(parts[parts.index("healthz") + 1 :])
        if self.headers.get("Upgrade", "").lower() == "websocket":
            return self.websocket()
        self.reply(
            200,
            {"host": self.headers.get("Host"), "path": url.path, "query": url.query},
        )

    do_HEAD = do_GET

    def health(self, action):
        # /healthz reports the state, /healthz/fail[/<seconds>] marks this
        # pool on this VM unhealthy (default 300 s), /healthz/ok recovers it.
        now = time.time()
        with LOCK:
            if action[:1] == ["fail"]:
                seconds = FAIL_SECONDS
                if len(action) > 1 and action[1].isdecimal():
                    seconds = min(int(action[1]), 3600)
                UNHEALTHY_UNTIL[self.port] = now + seconds
            elif action[:1] == ["ok"]:
                UNHEALTHY_UNTIL[self.port] = 0.0
            remaining = max(0, int(UNHEALTHY_UNTIL[self.port] - now))
        healthy = remaining == 0
        status = 200 if healthy or action else 503
        self.reply(status, {"healthy": healthy, "unhealthy_for_s": remaining})

    def websocket(self):
        key = self.headers.get("Sec-WebSocket-Key", "")
        accept = base64.b64encode(
            hashlib.sha1((key + WS_GUID).encode()).digest()
        ).decode()
        self.send_response(101)
        self.send_header("Upgrade", "websocket")
        self.send_header("Connection", "Upgrade")
        self.send_header("Sec-WebSocket-Accept", accept)
        self.end_headers()
        self.close_connection = True
        greeting = {
            "pool": self.pool,
            "backend": HOSTNAME,
            "port": int(self.port),
            "message": "websocket established",
        }
        self.send_frame(0x1, json.dumps(greeting).encode())
        while True:
            header = self.rfile.read(2)
            if len(header) < 2:
                return
            fin = header[0] & 0x80
            opcode = header[0] & 0x0F
            length = header[1] & 0x7F
            if length == 126:
                length = struct.unpack("!H", self.rfile.read(2))[0]
            elif length == 127:
                length = struct.unpack("!Q", self.rfile.read(8))[0]
            if length > 65536:
                self.send_frame(0x8, struct.pack("!H", 1009))
                return
            mask = self.rfile.read(4) if header[1] & 0x80 else bytes(4)
            payload = bytes(
                b ^ mask[i % 4] for i, b in enumerate(self.rfile.read(length))
            )
            if opcode == 0x8:
                self.send_frame(0x8, payload[:2])
                return
            if opcode == 0x9:
                self.send_frame(0xA, payload)
            elif opcode in (0x0, 0x1, 0x2):
                self.send_frame(opcode, payload, fin)

    def send_frame(self, opcode, payload, fin=0x80):
        header = bytes([fin | opcode])
        if len(payload) < 126:
            header += bytes([len(payload)])
        elif len(payload) < 65536:
            header += bytes([126]) + struct.pack("!H", len(payload))
        else:
            header += bytes([127]) + struct.pack("!Q", len(payload))
        self.wfile.write(header + payload)
        self.wfile.flush()


def serve(port):
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()


if __name__ == "__main__":
    threads = [
        threading.Thread(target=serve, args=(int(port),), daemon=True)
        for port in APPLICATIONS
    ]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
