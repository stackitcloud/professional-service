<!-- tags: ske, kubernetes, audit-log, kube-apiserver, observability, otel, telemetry-router, telemetry-link -->

# SKE kube-apiserver Audit Logs via Telemetry Router

This example enables Kubernetes API server audit logging on an SKE cluster and ships the
records through the **STACKIT Telemetry Router** into an **Observability instance**, where they
can be analysed with LogQL in Grafana.

A lot of logs are produced by the cluster itself, doing `get`/`list`/`watch` calls coming from kublet or gardener (infrastructure to manage your SKE). This can be overwhelming so this example aims to show how you can view relevant audit logs as well.

> ⚠️ **Private preview** > `audit = { enabled = true }` on `stackit_ske_cluster` is currently in private preview and is only
> accepted for enabled accounts. Please get in contact with STACKIT if you want to enable it.

## Prerequisites

- An existing STACKIT project attached to an SNA. This example only takes the project ID; it
  does not create the organization, folder, project or network area.
- A free subnet inside that SNA's network range for the SKE node network
- A service account key
- The SKE audit logs feature is enabled for your account, because this feature is in private review

## What gets created

<table>
  <thead>
    <tr>
      <th>File</th>
      <th>Resources</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>030-network.tf</code></td>
      <td>Routed node network for the cluster</td>
    </tr>
    <tr>
      <td><code>040-observability.tf</code></td>
      <td>Observability instance (Loki + Grafana) + ingest credential</td>
    </tr>
    <tr>
      <td><code>050-telemetry-router.tf</code></td>
      <td>Telemetry Router, access token, and the destinations incl. <strong>filters</strong></td>
    </tr>
    <tr>
      <td><code>060-telemetry-link.tf</code></td>
      <td>Telemetry Link — without it nothing is forwarded, not even in-project</td>
    </tr>
    <tr>
      <td><code>070-ske-cluster.tf</code></td>
      <td>SKE cluster with <code>audit = { enabled = true }</code> + ephemeral kubeconfig</td>
    </tr>
    <tr>
      <td><code>080-canary-workload.tf</code></td>
      <td>Namespace + ConfigMap that put findable records into the audit stream</td>
    </tr>
  </tbody>
</table>

## STACKIT Observability or STACKIT Logs

Both are Loki underneath. STACKIT Logs focuses only on providing a managed Loki and Observability includes a managed Grafana on top.
In this case it's easier for testing purposes.

A third variant worth knowing: the Telemetry Router can also write to S3 Object Storage

## Filtering

### Layer 1 — Source (SKE)

`audit = { enabled = true }` is an **on/off** switch only. The audit policy is managed by SKE and
cannot be tuned in any way, so you cannot tell the kube-apiserver to skip `watch` calls
from `system:serviceaccount:kube-system:*` for example.

### Layer 2 — Telemetry Router instance filter (global)

`stackit_telemetryrouter_instance` accepts a `filter` block that drops records before they reach
any destination. In this case we leave it open.

### Layer 3 — Telemetry Router Destination filter (routing)

`stackit_telemetryrouter_destination` takes a per-destination `filter`.
STACKIT sets on every forwarded record (dots become underscores when they arrive as Loki labels):

<table>
  <thead>
    <tr>
      <th>Attribute</th>
      <th>Loki label</th>
      <th>Example value</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>service.name</code></td>
      <td><code>service_name</code></td>
      <td><code>ske</code></td>
    </tr>
    <tr>
      <td><code>service.instance.id</code></td>
      <td><code>service_instance_id</code></td>
      <td><code>audit-demo</code></td>
    </tr>
    <tr>
      <td><code>stackit.log.kind</code></td>
      <td><code>stackit_log_kind</code></td>
      <td><code>kubernetes-audit</code></td>
    </tr>
    <tr>
      <td><code>stackit.log.type</code></td>
      <td><code>stackit_log_type</code></td>
      <td><code>SERVICE</code></td>
    </tr>
    <tr>
      <td><code>stackit.resource.id</code></td>
      <td><code>stackit_resource_id</code></td>
      <td><em>project UUID</em></td>
    </tr>
    <tr>
      <td><code>stackit.resource.type</code></td>
      <td><code>stackit_resource_type</code></td>
      <td><code>PROJECT</code></td>
    </tr>
    <tr>
      <td><code>stackit.visibility</code></td>
      <td><code>stackit_visibility</code></td>
      <td><code>PUBLIC</code></td>
    </tr>
    <tr>
      <td><code>cloud.region</code></td>
      <td><code>cloud_region</code></td>
      <td><code>eu01</code></td>
    </tr>
  </tbody>
</table>

So on this level we cannot filter on the JSON-formatted content / log line of the kube-apiserver audit logs itself. In this case we just filter on our cluster name and `kubernetes-audit`.

### Layer 4 — Query time, where the kublet/gardener noise actually can be filtered

Open the `grafana_url` output. Get credentials via portal.

**1. Everything (this is the firehose — expect ~250 records/minute)**

```logql
{service_name="ske", service_instance_id="audit-demo"}
```

**2. Human activity only**

```logql
{service_name="ske", stackit_log_kind="kubernetes-audit", service_instance_id="audit-demo"}
  | json user="user.username", verb="verb", res="objectRef.resource", ns="objectRef.namespace", stage="stage"
  | user !~ `system:.*`
  | user !~ `gardener\.cloud:.*`
```

## How to use

```bash
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform apply
```

The apply already produces findable audit records: the canary Namespace and ConfigMap are created

## Cleanup

```bash
terraform destroy
```
