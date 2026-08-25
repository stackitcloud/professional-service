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

resource "kubernetes_namespace_v1" "kvm_validator" {
  metadata {
    name = "kvm-validator"
    labels = {
      "app.kubernetes.io/managed-by" = "terraform"
    }
  }
}

# One pod per node across all three pools. Never exits so results stay in logs.
# kubectl -n kvm-validator get pods -o wide
# kubectl -n kvm-validator logs -l app.kubernetes.io/name=kvm-validator
resource "kubernetes_daemon_set_v1" "kvm_validator" {
  metadata {
    name      = "kvm-validator"
    namespace = kubernetes_namespace_v1.kvm_validator.metadata[0].name
    labels = {
      "app.kubernetes.io/name"       = "kvm-validator"
      "app.kubernetes.io/managed-by" = "terraform"
    }
  }

  spec {
    selector {
      match_labels = {
        "app.kubernetes.io/name" = "kvm-validator"
      }
    }

    template {
      metadata {
        labels = {
          "app.kubernetes.io/name" = "kvm-validator"
        }
      }

      spec {
        node_selector = {
          "nested-virt-demo" = "true"
        }

        container {
          name  = "kvm-validator"
          image = "alpine:3"

          security_context {
            privileged = true
          }

          command = ["/bin/sh", "-c"]
          args = [<<-EOT
            set -eu

            NODE=$NODE_NAME
            # Parse pool name from node name: shoot--<id>--<cluster>-<pool>-z<az>-<hash>-<suffix>
            POOL=$(echo "$NODE" | sed 's/.*-nested-virt-\(.*\)-z[0-9].*/\1/')
            PASS=0
            FAIL=0

            check() {
              LABEL=$1; STATUS=$2; MSG=$3
              if [ "$STATUS" = "pass" ]; then
                printf "  [PASS] %-22s %s\n" "$LABEL" "$MSG"
                PASS=$((PASS+1))
              else
                printf "  [FAIL] %-22s %s\n" "$LABEL" "$MSG"
                FAIL=$((FAIL+1))
              fi
            }

            echo "=== Nested Virtualisation Smoke Test ==="
            echo "Node      : $NODE"
            echo "Pool      : $POOL"
            echo "Kernel    : $(uname -r)"
            echo "Timestamp : $(date -u +%Y-%m-%dT%H:%M:%SZ)"
            echo ""

            if [ -c /dev/kvm ]; then
              check "/dev/kvm present"  pass "$(ls -la /dev/kvm)"
            else
              check "/dev/kvm present"  fail "device not found"
            fi

            if grep -qw vmx /proc/cpuinfo 2>/dev/null; then
              check "CPU virt flag" pass "Intel VT-x (vmx)"
            elif grep -qw svm /proc/cpuinfo 2>/dev/null; then
              check "CPU virt flag" pass "AMD-V (svm)"
            else
              check "CPU virt flag" fail "no vmx or svm in /proc/cpuinfo"
            fi

            if [ -c /dev/kvm ] && [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
              check "/dev/kvm access" pass "read/write confirmed"
            else
              check "/dev/kvm access" fail "not accessible"
            fi

            echo ""
            if [ "$FAIL" -eq 0 ]; then
              RESULT="PASS"
              echo "Result: PASS — nested virtualisation is operational"
            else
              RESULT="FAIL"
              echo "Result: FAIL — this flavor does not expose /dev/kvm"
              echo "        Use a g2 or g3 node pool for KVM workloads."
            fi
            echo ""

            while true; do
              echo "[$(date -u +%H:%M:%SZ)] pool=$POOL node=$NODE result=$RESULT"
              sleep 60
            done
          EOT
          ]

          env {
            name = "NODE_NAME"
            value_from {
              field_ref {
                field_path = "spec.nodeName"
              }
            }
          }
          resources {
            requests = {
              cpu    = "50m"
              memory = "32Mi"
            }
            limits = {
              cpu    = "100m"
              memory = "64Mi"
            }
          }

          # Surfaces the KVM difference in `kubectl get pods` READY column.
          liveness_probe {
            exec {
              command = ["/bin/sh", "-c", "test -c /dev/kvm"]
            }
            initial_delay_seconds = 15
            period_seconds        = 30
            failure_threshold     = 2
          }
        }

      }
    }
  }
}
