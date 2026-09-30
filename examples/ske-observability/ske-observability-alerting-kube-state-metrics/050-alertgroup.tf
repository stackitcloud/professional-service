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

resource "stackit_observability_alertgroup" "example" {
  project_id  = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  instance_id = stackit_observability_instance.example.instance_id
  name        = "TestAlertGroup"
  interval    = "2h"
  rules = [
    {
      alert      = "SimplePodCheck"
      expression = "sum(kube_pod_status_phase{phase=\"Running\", namespace=\"example\"}) > 0"
      for        = "60s"
      labels = {
        severity = "critical"
      },
      annotations = {
        summary     = "Test Alert is working"
        description = "Test Alert"
      }
    },
  ]
}
