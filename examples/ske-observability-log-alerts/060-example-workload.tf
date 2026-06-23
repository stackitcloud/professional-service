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

resource "kubernetes_namespace" "example" {
  metadata {
    name = "example"
  }
}

resource "kubernetes_pod" "logger" {
  metadata {
    name      = "logger"
    namespace = kubernetes_namespace.example.metadata[0].name
    labels = {
      app = "logger"
    }
  }

  spec {
    container {
      name  = "logger"
      image = "bash"
      command = [
        "bash",
        "-c",
        <<EOF
        while true; do
          sleep $(shuf -i 1-3 -n 1)
          echo "ERROR: $(date) - Simulated error message $(shuf -i 1-100 -n 1)" 1>&2
        done
        EOF
      ]
    }
  }
}
