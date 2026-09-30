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

"""Demo workload: a HTTP service that calls itself.

There is no OpenTelemetry code here on purpose. The spans come from
auto-instrumentation of Flask and requests.
"""

import logging
import random
import threading
import time

import requests
from flask import Flask

app = Flask(__name__)
logging.basicConfig(level=logging.INFO)
log = logging.getLogger("demo")


@app.get("/orders")
def orders():
    time.sleep(random.uniform(0.005, 0.04))
    if random.random() < 0.2:
        log.error("Simulated error message %d", random.randint(1, 100))
        return {"error": "order lookup failed"}, 500
    return {"orders": random.randint(1, 100)}


def caller():
    time.sleep(2)
    while True:
        try:
            requests.get("http://127.0.0.1:8080/orders", timeout=5)
        except requests.RequestException as exc:
            log.warning("request failed: %s", exc)
        time.sleep(random.randint(1, 3))


threading.Thread(target=caller, daemon=True).start()
app.run(host="0.0.0.0", port=8080)
