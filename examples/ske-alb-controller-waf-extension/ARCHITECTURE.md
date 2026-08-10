# Architecture SKE ALB Extension with WAF

## L7 Load Balancer Kubernetes Integration

The STACKIT ALB Controller implements the standard Kubernetes Ingress controller pattern: it watches `IngressClass` and `Ingress` objects and translates them into a fully managed STACKIT Application Load Balancer. With the SKE extension, STACKIT deploys and manages the controller itself no manual Pod deployment, RBAC, or credential rotation is required.

```mermaid
flowchart TB
    TF(["Terraform"])

    subgraph STACKIT["STACKIT Platform"]
        direction TB
        WAF["WAF\nOWASP CRS + custom rules"]
        ALB["Application Load Balancer\nmanaged L7 HTTPS termination"]
        WAF --> ALB
    end

    subgraph K8s["Kubernetes Cluster (SKE)"]
        direction TB
        subgraph ext["ALB Extension managed by STACKIT"]
            Ctrl["ALB Controller\ncredentials managed by STACKIT"]
        end

        IC["IngressClass: stackit-alb\nnetwork-mode: NodePort\nweb-application-firewall-name: &lt;waf-config&gt;"]

        subgraph ns["Namespace: alb-demo"]
            Ing["Ingress\nhost: app.example.com\ntls: hello-tls"]
            Svc["Service\ntype: NodePort"]
            Pod["Pod\nnginxdemos/hello"]
        end
    end

    Internet(["Internet"]) --> WAF
    ALB -- "NodePort" --> Svc
    Svc --> Pod

    IC -- "watched by" --> Ctrl
    Ing -- "watched by" --> Ctrl
    Ing -- "uses" --> IC
    Ctrl -- "provisions via\nSTACKIT API" --> ALB

    TF -. "stackit_alb_waf_*" .-> WAF
    TF -. "kubernetes_ingress_class_v1" .-> IC
    TF -. "kubernetes_ingress_v1\nkubernetes_service_v1\nkubernetes_deployment_v1" .-> ns
```

## Request flow

1. **Client** sends an HTTPS request to the ALB public IP.
2. **WAF** inspects the request against OWASP CRS rules and any custom rules; blocks matching requests with HTTP 403.
3. **ALB** terminates TLS, matches the `Ingress` host/path rules, and forwards traffic to the node on a NodePort.
4. **NodePort Service** routes the request to a healthy `Pod`.

## WAF attachment

The WAF configuration is attached at the `IngressClass` level via the `alb.stackit.cloud/web-application-firewall-name` annotation. This means every `Ingress` that references this `IngressClass` is automatically protected, no per-Ingress annotation needed.

## Key resources

| Resource                             | Layer      | Managed by                       |
| ------------------------------------ | ---------- | -------------------------------- |
| `stackit_alb_waf_managed_rule_set`   | STACKIT    | Terraform                        |
| `stackit_alb_waf_custom_rule_group`  | STACKIT    | Terraform                        |
| `stackit_alb_waf_configuration`      | STACKIT    | Terraform                        |
| Application Load Balancer            | STACKIT    | ALB Controller (via STACKIT API) |
| ALB Controller                       | Kubernetes | STACKIT (SKE extension)          |
| `IngressClass`                       | Kubernetes | Terraform                        |
| `Ingress` / `Service` / `Deployment` | Kubernetes | Terraform                        |
