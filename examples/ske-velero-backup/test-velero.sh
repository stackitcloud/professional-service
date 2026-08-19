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

# 1. Create test resources
kubectl create namespace test
kubectl create secret generic my-secret --from-literal=password=supersecret -n test
kubectl get secret my-secret -n test

# 2. Backup the namespace
velero backup create test-backup --include-namespaces test --wait

# 3. Confirm backup is complete
velero backup describe test-backup

# 4. Delete the secret
kubectl delete secret my-secret -n test
kubectl get secret my-secret -n test  # should return "not found"

# 5. Restore
velero restore create --from-backup test-backup --include-namespaces test --wait

# 6. Confirm the secret is back
kubectl get secret my-secret -n test
kubectl get secret my-secret -n test -o jsonpath='{.data.password}' | base64 -d
