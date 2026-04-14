# How to Forward the Real Client IP to the Ingress Controller

When your application is accessed through a Load Balancer, the original client IP may not be visible to your pods unless the **TCP Proxy Protocol** is enabled and properly configured.
STACKIT supports Proxy Protocol version 2. Please follow the steps below to ensure your application and ingress-nginx controller can correctly receive and log the original client IP.

**Reference:**
Official STACKIT documentation: [Load Balancer SKE - TCP Proxy Protocol](https://docs.stackit.cloud/stackit/en/load-balancer-ske-28476594.html#LoadBalancerSKE-TCPProxyProtocol)

---

## 1. Configure the Service for Proxy Protocol

The Kubernetes `Service` of type `LoadBalancer` must be annotated to enable Proxy Protocol support from STACKIT.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-proxy-service
  annotations:
    lb.stackit.cloud/tcp-proxy-protocol: "true"
spec:
  selector:
    app: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
  type: LoadBalancer
```

**Note:**
Make sure the application behind the Load Balancer is compatible with Proxy Protocol v2. If not, connections might fail or behave unexpectedly.

---

## 2. Enable Proxy Protocol on ingress-nginx

Your ingress controller must also be configured to accept Proxy Protocol headers.
This may require special Helm values when deploying ingress-nginx.

- See how this is done in [07-helm.tf](07-helm.tf) for this repository.
- In general, for ingress-nginx Helm charts, you will need:
  - `controller.config.use-proxy-protocol: "true"`
  - Any additional Service annotations as required by your cloud provider and use case.

---

## 3. Testing

To load test your endpoint and validate real client IP handling, you can use [`fortio`](https://fortio.org/):

```sh
fortio load --qps 50 -t 10s <url from outputs.tf>
```

Replace `<url from outputs.tf>` with the Load Balancer or Ingress endpoint output by your Terraform configuration.

---

## Important

- Proxy Protocol must be enabled on **both** the Load Balancer service **and** the ingress/nginx controller.
- If either is not configured, the original client IP will not be visible within your pods, and may result in errors.
- Always review the official documentation for your platform and ingress-nginx version.

---
