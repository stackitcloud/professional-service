<!-- tags: ske, observability, logging, alerting, alloy, kubernetes -->

# SKE Observability Log-Alerts

## Overview

This guide walks you through setting up log-based alerting in STACKIT Observability using Grafana Alloy to ship Kubernetes logs.

Alloy runs as a DaemonSet on the SKE cluster, tails the container logs the kubelet writes to
`/var/log/pods`, labels them with namespace, pod and container, and pushes them to the Loki
endpoint of an Observability instance. A log alert group then fires on a pattern in those logs.

## Why Alloy

Any agent that speaks the Loki push API can fill this role.
[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) and Fluent Bit are the usual
alternatives, and the Observability instance also accepts OTLP directly through its
`otlp_http_logs_url`.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform apply
```

## Query Logs

In the Grafana instance that comes with Observability:

```logql
{namespace="example", pod="logger"}
```

## Alerts

The alert group in `060-log-alertgroup.tf` fires once that stream contains
`Simulated error message`, and the Alertmanager routing in `040-observability.tf` sends the
notification by email.
