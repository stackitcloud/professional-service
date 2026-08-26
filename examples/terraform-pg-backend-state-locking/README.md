<!-- tags: terraform, postgresql, state, backend, remote-state, locking -->

# STACKIT Terraform PostgreSQL Backend with State Locking

This repository demonstrates how to configure STACKIT PostgreSQL Flex as a Terraform backend to enable remote state storage and native state locking.

To resolve the circular dependency of provisioning a state backend using Terraform, the deployment is split into two isolated stages:

1. **`00-bootstrap/`**: Provisions the backend infrastructure (PostgreSQL Flex instance, database and service user).
2. **`01-example/`**: Represents the primary infrastructure, utilizing the provisioned PostgreSQL database as its remote backend.

---

**⚠️ Security Notice:** The PostgreSQL Flex instance in `00-bootstrap/` is configured with an open ACL (`0.0.0.0/0`) for development convenience. Before deploying to production, restrict the ACL to your specific egress IP ranges to prevent the database from being accessible via the public internet.
