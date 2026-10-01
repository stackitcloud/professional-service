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


import argparse
import base64
import json
import os
import ssl
import threading
import time
from datetime import datetime, timedelta, timezone
from functools import partial
from http.server import (
    BaseHTTPRequestHandler,
    SimpleHTTPRequestHandler,
    ThreadingHTTPServer,
)

MAIL = "costguard-vmtest@sa.stackit.cloud"


def b64(obj):
    return base64.urlsafe_b64encode(json.dumps(obj).encode()).rstrip(b"=").decode()


def fake_token():
    now = int(time.time())
    return ".".join(
        [
            b64({"alg": "RS256", "typ": "JWT"}),
            b64({"iss": "fake", "sub": MAIL, "iat": now, "exp": now + 3600}),
            "c2ln",
        ]
    )


class Metadata(BaseHTTPRequestHandler):
    def send(self, code, body):
        data = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        self.server.log.write(f"{time.strftime('%H:%M:%S')} metadata {self.path}\n")
        if self.path == "/stackit/v1/service-accounts":
            return self.send(200, {"serviceAccountMails": [MAIL]})
        if self.path in (
            f"/stackit/v1/service-accounts/{MAIL}/token",
            "/stackit/v1/service-accounts/costguard-vmtest%40sa.stackit.cloud/token",
        ):
            until = (datetime.now(timezone.utc) + timedelta(hours=1)).isoformat(
                timespec="microseconds"
            )
            return self.send(
                200, {"token": fake_token(), "validUntil": until.replace("+00:00", "Z")}
            )
        return self.send(
            404,
            {
                "code": "404 Not Found",
                "message": "The resource could not be found.",
                "title": "Not Found",
            },
        )

    def log_message(self, *args):
        pass


class Webhook(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        try:
            payload = json.loads(body)
        except ValueError:
            payload = {"invalid_json": body.decode("utf-8", "replace")}
        with self.server.lock, open(self.server.out, "a", encoding="utf-8") as f:
            f.write(
                json.dumps(
                    {
                        "time": time.strftime("%H:%M:%S"),
                        "path": self.path,
                        "payload": payload,
                    }
                )
                + "\n"
            )
        self.send_response(200)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def log_message(self, *args):
        pass


class Quiet(SimpleHTTPRequestHandler):
    def log_message(self, fmt, *args):
        self.server.log.write(
            f"{time.strftime('%H:%M:%S')} {self.server.name} " + fmt % args + "\n"
        )


def serve(server, name, log):
    server.name = name
    server.log = log
    threading.Thread(target=server.serve_forever, daemon=True).start()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--dir", required=True, help="work directory with seed/, dist/, tls/"
    )
    ap.add_argument("--metadata-port", type=int, required=True)
    ap.add_argument("--seed-port", type=int, required=True)
    ap.add_argument("--download-port", type=int, required=True)
    ap.add_argument("--webhook-port", type=int, required=True)
    args = ap.parse_args()

    log = open(
        os.path.join(args.dir, "services.log"), "a", buffering=1, encoding="utf-8"
    )
    tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    tls.load_cert_chain(
        os.path.join(args.dir, "tls", "server.pem"),
        os.path.join(args.dir, "tls", "server.key"),
    )

    serve(
        ThreadingHTTPServer(("127.0.0.1", args.metadata_port), Metadata),
        "metadata",
        log,
    )
    serve(
        ThreadingHTTPServer(
            ("127.0.0.1", args.seed_port),
            partial(Quiet, directory=os.path.join(args.dir, "seed")),
        ),
        "seed",
        log,
    )

    downloads = ThreadingHTTPServer(
        ("127.0.0.1", args.download_port),
        partial(Quiet, directory=os.path.join(args.dir, "downloads")),
    )
    downloads.socket = tls.wrap_socket(downloads.socket, server_side=True)
    serve(downloads, "downloads", log)

    webhook = ThreadingHTTPServer(("127.0.0.1", args.webhook_port), Webhook)
    webhook.socket = tls.wrap_socket(webhook.socket, server_side=True)
    webhook.out = os.path.join(args.dir, "webhook.jsonl")
    webhook.lock = threading.Lock()
    serve(webhook, "webhook", log)

    log.write(f"{time.strftime('%H:%M:%S')} services up\n")
    while True:
        time.sleep(3600)


if __name__ == "__main__":
    main()
