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
