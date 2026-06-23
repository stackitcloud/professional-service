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

resource "stackit_observability_logalertgroup" "example" {
  project_id  = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  instance_id = stackit_observability_instance.example.instance_id
  name        = "TestLogAlertGroup"
  interval    = "1m"
  rules = [
    {
      alert      = "SimplePodLogAlertCheck"
      expression = "sum(rate({namespace=\"example\", pod=\"logger\"} |= \"Simulated error message\" [1m])) > 0"
      for        = "60s"
      labels = {
        severity = "critical"
      },
      annotations = {
        summary : "Test Log Alert is working"
        description : "Test Log Alert"
      },
    },
  ]
}
