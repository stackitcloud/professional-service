<!-- tags: ske, kubernetes, nested-virtualization, kvm, compute, node-pool, terraform -->

# SKE Nested Virtualisation

Deploys three node pools and runs a KVM smoke test on each to show which STACKIT flavors expose `/dev/kvm`.

| Pool             | flavor            | Expected |
| ---------------- | ----------------- | -------- |
| `g3-intel`       | g3i.2 — Intel VMX | PASS     |
| `g2-intel`       | g2i.8 — Intel VMX | PASS     |
| `c2a-amd-no-kvm` | c2a.2d — AMD      | FAIL     |

## Supported flavors

Only **g2 Intel and g3 Intel** nodes expose `/dev/kvm`. This reflects the current STACKIT SKE configuration and may change in future.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
# fill in project_id and stackit_service_account_key_path
terraform init && terraform apply
```

## Results

See [RESULTS.md](RESULTS.md)

```bash
kubectl -n kvm-validator get pods -o wide

# Side-by-side results
kubectl -n kvm-validator logs -l app.kubernetes.io/name=kvm-validator --prefix
```
