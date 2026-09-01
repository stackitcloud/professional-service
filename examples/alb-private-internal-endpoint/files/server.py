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

# HTTPS backend for load balancer TLS bridging tests. The application listens
# on one port per certificate and every response names the VM, the port, the
# TLS version and the fingerprint of the certificate that served the request.
# Usage: server.py <port>=<certificate file>:<key file> [...]
import hashlib
import json
import socket
import ssl
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit

HOSTNAME = socket.gethostname()


def fingerprint(certificate_file):
    with open(certificate_file) as handle:
        der = ssl.PEM_cert_to_DER_cert(handle.read())
    digest = hashlib.sha256(der).hexdigest().upper()
    return ":".join(digest[i : i + 2] for i in range(0, len(digest), 2))


class Server(ThreadingHTTPServer):
    def handle_error(self, request, client_address):
        # A failed TLS handshake, for example a plain-text probe, is not worth
        # a traceback in the journal.
        pass


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def setup(self):
        super().setup()
        # The handshake runs in the thread of the connection, so a client that
        # never sends anything does not stall the accept loop of the port.
        self.connection.do_handshake()

    def log_message(self, fmt, *args):
        # Silence the per-request log of BaseHTTPRequestHandler; the health
        # checks of the load balancer alone would add a line every few seconds.
        pass

    def do_GET(self):
        path = urlsplit(self.path).path
        body = {
            "backend": HOSTNAME,
            "port": self.server.server_address[1],
            "path": path,
            "tls_version": self.connection.version(),
            "cipher": self.connection.cipher()[0],
            "certificate_sha256": self.server.certificate_fingerprint,
        }
        if path == "/healthz":
            body["healthy"] = True
        payload = (json.dumps(body) + "\n").encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(payload)

    do_HEAD = do_GET


def build(port, certificate_file, key_file):
    server = Server(("0.0.0.0", port), Handler)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(certificate_file, key_file)
    server.socket = context.wrap_socket(
        server.socket, server_side=True, do_handshake_on_connect=False
    )
    server.certificate_fingerprint = fingerprint(certificate_file)
    return server


if __name__ == "__main__":
    # Bind every port and load every certificate before serving, so that a
    # port or file that is not usable ends the process and systemd restarts it.
    servers = []
    for argument in sys.argv[1:]:
        port, files = argument.split("=", 1)
        certificate_file, key_file = files.split(":", 1)
        servers.append(build(int(port), certificate_file, key_file))
    threads = [
        threading.Thread(target=server.serve_forever, daemon=True) for server in servers
    ]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
