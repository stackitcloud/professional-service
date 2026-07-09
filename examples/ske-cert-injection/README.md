# Kubernetes Custom CA Injection Blueprint

This project is a proof of concept for injecting a custom Root CA into a Kubernetes cluster at two levels:

1. Node level, by patching each worker node trust store with a privileged DaemonSet.
2. Pod level, by distributing a trusted CA bundle to workloads with cert-manager trust-manager.

The project also includes a fake HTTPS endpoint so the full workflow can be tested without a real corporate proxy such as Zscaler.

## Approaches

### Node-Level (DaemonSet Injector)

**Use case:** Trust the custom CA for everything running on the node — including containerd image pulls, node-local daemons, and any workload that relies on the OS trust store rather than a mounted bundle.

**How it works:** A privileged DaemonSet schedules one Pod per node. The Pod copies the CA certificate onto the host filesystem via a `hostPath` mount and then executes the host's trust-store update tool (`update-ca-certificates` on Debian/Ubuntu, `update-ca-trust extract` on RHEL/CentOS, or a direct bundle append for Flatcar). containerd is restarted asynchronously so the new trust store takes effect without killing the injector Pod.

**Technologies:** Kubernetes DaemonSet, `hostPath` volume, `hostPID: true`, `privileged: true`, `nsenter`/`chroot` for host namespace entry.

**Trade-off:** Broad host access makes this powerful but operationally sensitive. It is suitable for a PoC or an environment where node-bootstrap tooling is unavailable. For production, prefer baking the CA into the node image or using cloud-init.

### Pod-Level (trust-manager Bundle)

**Use case:** Distribute the custom CA to application workloads without touching the host. Pods that need to call internal HTTPS services mount the CA bundle from a ConfigMap and pass it to their HTTP client.

**How it works:** cert-manager's trust-manager reads the CA from a source ConfigMap in `kube-system` and replicates it as a `trusted-ca-bundle` ConfigMap into every namespace. Pods mount the ConfigMap key at the path their runtime or HTTP library reads (e.g. `/etc/ssl/certs/ca-certificates.crt` for Alpine/BusyBox `wget`).

**Technologies:** [cert-manager](https://cert-manager.io) (webhook infrastructure), [trust-manager](https://cert-manager.io/docs/trust/trust-manager/) Bundle CRD, Kubernetes ConfigMap, `volumeMount` with `subPath`.

**Trade-off:** No host privileges required and fully auditable via standard Kubernetes objects. The CA is only trusted by pods that explicitly mount the bundle — it does not cover containerd or node-level traffic.

## Scope

The blueprint proves this flow:

1. Generate a fake Root CA.
2. Generate a TLS server certificate signed by that Root CA for `dummy-server.default.svc.cluster.local`.
3. Run an HTTPS Nginx Pod that presents that unknown certificate.
4. Inject the fake Root CA into Kubernetes nodes.
5. Use trust-manager to distribute the fake Root CA as a ConfigMap to all namespaces.
6. Run one test Pod without the CA and prove TLS fails.
7. Run one test Pod with the mounted CA bundle and prove TLS succeeds.

This is a PoC, not a production hardening guide. The node-level injector is privileged and mutates host filesystems.

## Project Layout

```text
ske-cert-injection/
├── 00-simulation/
│   ├── generate-certs.sh
│   └── dummy-https-server.yaml
├── 01-node-level/
│   ├── configmap-ca.yaml
│   └── daemonset-injector.yaml
├── 02-pod-level/
│   ├── install-trust-manager.sh
│   └── bundle.yaml
├── 03-test-clients/
│   ├── client-fail.yaml
│   └── client-success.yaml
├── deploy.sh
├── verify.sh
├── test-node-autoscaling.sh
├── test-pod-scaling.sh
└── MAINTAINERS.md
```

Generated certificate artifacts may also exist in `00-simulation/` after running the scripts:

- `fake-ca.crt`
- `fake-ca.key`
- `fake-ca.srl`
- `tls.crt`
- `tls.key`

## Prerequisites

You need:

- `kubectl` configured for the target cluster.
- `helm`.
- `openssl`.
- Permission to create resources in `default`, `kube-system`, and `cert-manager`.
- A cluster that permits privileged DaemonSets with `hostPath` mounts.

## Deployment

Run the full rollout:

```bash
./deploy.sh
```

The script performs the deployment in dependency order:

1. Generates the fake CA and server certificate.
2. Creates or updates the `default/dummy-tls` Secret.
3. Renders `01-node-level/configmap-ca.yaml` from the generated Root CA.
4. Deploys the dummy HTTPS server.
5. Deploys the node-level CA injector DaemonSet.
6. Installs cert-manager.
7. Installs trust-manager with `app.trust.namespace=kube-system`.
8. Applies the trust-manager Bundle.
9. Deploys the two test clients.

Then run verification:

```bash
./verify.sh
```

## Verification Behavior

`verify.sh` checks:

- `node-ca-injector` DaemonSet has `READY == DESIRED`.
- `trusted-ca-bundle` Bundle reports `Synced=True`.
- `client-fail` and `client-success` are Ready before tests run.
- `client-fail` fails to fetch `https://dummy-server.default.svc.cluster.local`.
- `client-success` succeeds against the same URL.

The negative test intentionally expects `wget` to fail. The script captures that exit code without allowing `set -e` to terminate the run prematurely.

## Problems Solved

### No Real Proxy Available

The cluster does not need a real Zscaler or corporate TLS inspection proxy. `00-simulation/generate-certs.sh` creates a fake Root CA and a server certificate for the in-cluster DNS name `dummy-server.default.svc.cluster.local`. `dummy-https-server.yaml` runs Nginx over HTTPS using that certificate.

This gives a deterministic TLS failure until the fake Root CA is trusted.

### Static YAML Cannot Embed a Certificate Before It Exists

`configmap-ca.yaml` is generated dynamically. The cert generation script uses:

```bash
kubectl create configmap custom-ca-store \
  --namespace kube-system \
  --from-file=fake-ca.crt=... \
  --dry-run=client \
  -o yaml
```

That guarantees the ConfigMap contains the exact generated CA data.

### trust-manager Source Namespace

The source ConfigMap lives in `kube-system`. trust-manager only reads trust sources from its configured trust namespace, so the Helm install explicitly sets:

```bash
--set app.trust.namespace=kube-system
```

Without this, the Bundle may exist but fail to read `custom-ca-store`.

### cert-manager and trust-manager Race Conditions

trust-manager depends on cert-manager readiness. `install-trust-manager.sh` waits for cert-manager deployments and Pods before installing trust-manager. It then waits for trust-manager readiness before continuing.

This avoids webhook and CRD timing failures during automated rollout.

### Runtime Package Installation Trap

The first DaemonSet version used `apk add util-linux ca-certificates` inside the Alpine container to install `nsenter`. On the STACKIT cluster, DNS resolution to Alpine package mirrors failed:

```text
WARNING: fetching https://dl-cdn.alpinelinux.org/... DNS: transient error
ERROR: unable to select packages
```

That caused the injector Pod to exit before touching the host. The current DaemonSet does not install packages at runtime. It mounts the host root at `/host` and uses:

- container `nsenter` if available
- `/host/usr/bin/nsenter` if available
- `/host/bin/nsenter` if available
- `chroot /host /bin/sh` as a final fallback

### Flatcar and Hardened Node Filesystem Differences

STACKIT worker nodes may be Flatcar Container Linux or hardened Ubuntu. Trust paths differ by OS, and Flatcar commonly has read-only system areas.

The injector now attempts several trust locations:

- `/usr/local/share/ca-certificates`
- `/etc/pki/ca-trust/source/anchors`
- `/etc/ssl/certs`

It then runs whichever host trust refresh tool exists:

- `update-ca-certificates`
- `update-ca-trust extract`

If neither exists, it attempts direct bundle append for common bundle files:

- `/etc/ssl/certs/ca-certificates.crt`
- `/etc/pki/tls/certs/ca-bundle.crt`

Read-only or missing paths are logged and skipped instead of crashing the Pod.

### Containerd Suicide Trap

Restarting `containerd` synchronously from inside the injector Pod can kill the Pod before it reaches `sleep infinity`. If that happens, the DaemonSet never reports Ready, and `verify.sh` fails with:

```text
FAIL: node-ca-injector READY=0, DESIRED=1.
```

The current DaemonSet schedules the restart asynchronously:

```sh
nohup sh -c 'sleep 10; systemctl restart containerd || true' >/tmp/node-ca-injector-containerd-restart.log 2>&1 &
```

That gives the injector process time to finish and keep the Pod alive. A sentinel file at `/var/lib/node-ca-injector/containerd-restart-scheduled` prevents repeated restarts.

## Troubleshooting

Check the injector Pod:

```bash
kubectl get pods -n kube-system -l app=node-ca-injector -o wide
```

Fetch logs:

```bash
POD="$(kubectl get pods -n kube-system -l app=node-ca-injector -o jsonpath='{.items[0].metadata.name}')"
kubectl logs -n kube-system "$POD" -c injector
kubectl logs -n kube-system "$POD" -c injector --previous
kubectl describe pod -n kube-system "$POD"
```

Check node OS details:

```bash
NODE="$(kubectl get pod -n kube-system "$POD" -o jsonpath='{.spec.nodeName}')"
kubectl get node "$NODE" -o wide
kubectl describe node "$NODE"
```

Check trust-manager:

```bash
kubectl get bundle trusted-ca-bundle
kubectl describe bundle trusted-ca-bundle
kubectl get configmap trusted-ca-bundle -n default -o yaml
```

Check the test Pods:

```bash
kubectl get pods client-fail client-success -n default
kubectl describe pod client-success -n default
kubectl exec client-fail -n default -- wget -q -O- https://dummy-server.default.svc.cluster.local
kubectl exec client-success -n default -- wget -q -O- https://dummy-server.default.svc.cluster.local
```

## Security Notes

The node-level injector requires:

- `privileged: true`
- `hostPID: true`
- `hostPath` mount of `/`
- host trust-store mutation
- optional container runtime restart

Those are intentionally broad permissions for a PoC. For production, prefer image-level CA baking, managed node bootstrap configuration, machine images, cloud-init, or a controlled node management pipeline. Pod-level distribution with trust-manager is usually safer and easier to reason about than mutating host trust stores from Kubernetes.

## Cleanup

Remove the test clients, Bundle, injector, ConfigMaps, and dummy server:

```bash
kubectl delete -f 03-test-clients/client-success.yaml --ignore-not-found
kubectl delete -f 03-test-clients/client-fail.yaml --ignore-not-found
kubectl delete -f 02-pod-level/bundle.yaml --ignore-not-found
kubectl delete -f 01-node-level/daemonset-injector.yaml --ignore-not-found
kubectl delete -f 01-node-level/configmap-ca.yaml --ignore-not-found
kubectl delete -f 00-simulation/dummy-https-server.yaml --ignore-not-found
kubectl delete secret dummy-tls -n default --ignore-not-found
```

This does not automatically remove CA files copied onto nodes or undo the asynchronous containerd restart sentinel. Host cleanup depends on node OS and should be handled through your node management process.
