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

output "folder_id" {
  description = "Container ID of the created folder"
  value       = stackit_resourcemanager_folder.showcase.id
}

output "project_id" {
  description = "UUID of the created project — use this for all subsequent STACKIT CLI commands"
  value       = stackit_resourcemanager_project.showcase.project_id
}

output "network_id" {
  description = "UUID of the private network"
  value       = stackit_network.workshop.network_id
}

output "vm_public_ip" {
  description = "Public IPv4 address of the Docker host"
  value       = stackit_public_ip.vm.ip
}

output "dns_zone_id" {
  description = "UUID of the DNS zone"
  value       = stackit_dns_zone.workshop.id
}

output "dns_primary_nameserver" {
  description = "Primary nameserver FQDN for the DNS zone — use for delegation and ACME DNS-01"
  value       = stackit_dns_zone.workshop.primary_name_server
}

output "ssh_command" {
  description = "SSH command to connect to the Docker host (Debian 12 default user)"
  value       = "ssh debian@${stackit_public_ip.vm.ip}"
}

output "nginx_test_url" {
  description = "URL to verify the nginx test container"
  value       = "http://${stackit_public_ip.vm.ip}"
}

# ─── ALB Outputs ──────────────────────────────────────────────────────────────

output "alb_public_ip" {
  description = "Public IP address of the ALB"
  value       = stackit_public_ip.alb.ip
}

output "alb_url" {
  description = "HTTP URL of the ALB (DNS must be delegated first)"
  value       = "http://${var.dns_name}"
}

output "alb_name" {
  description = "ALB name — use to look up the public IP: stackit load-balancer describe --name <name> --project-id <id>"
  value       = stackit_application_load_balancer.workshop.name
}

output "alb_private_address" {
  description = "ALB private (internal) IP address"
  value       = stackit_application_load_balancer.workshop.private_address
}

output "alb_target_security_group" {
  description = "Security group ID automatically assigned to ALB targets — already injected into VM NIC"
  value       = stackit_application_load_balancer.workshop.target_security_group
}

output "acme_next_steps" {
  description = "Phase 2 instructions: replace the bootstrap cert with a Let's Encrypt cert via stackit-acme-alb on the VM"
  value       = <<-EOT

    ═══════════════════════════════════════════════════════════════
     PHASE 2 — Let's Encrypt certificate via stackit-acme-alb
     VM: ${stackit_public_ip.vm.ip}  |  Domain: ${var.dns_name}
    ═══════════════════════════════════════════════════════════════

    PREREQUISITE: DNS delegation must be active before certbot can run.
    Set NS records for ${var.dns_name} to: $(terraform output -raw dns_primary_nameserver)
    Verify: dig NS ${var.dns_name}

    ── Step 1: Local — upload files to the VM ────────────────────

      scp -r stackit-acme-alb debian@${stackit_public_ip.vm.ip}:~/stackit-acme-alb
      scp keys/sa-key.json    debian@${stackit_public_ip.vm.ip}:~/stackit-acme-alb/sa-key.json

    ── Step 2: Local — encode the SA key as Base64 (macOS) ───────

      base64 -i keys/sa-key.json | tr -d '\n'

    ── Step 3: SSH to the VM and fill in .env ────────────────────

      ssh debian@${stackit_public_ip.vm.ip}
      cd ~/stackit-acme-alb

      sed -i "s|^PROJECT_ID=.*|PROJECT_ID=${stackit_resourcemanager_project.showcase.project_id}|" .env
      sed -i "s|^ALB_NAME=.*|ALB_NAME=${stackit_application_load_balancer.workshop.name}|" .env
      sed -i "s|^DOMAIN_WHITELIST=.*|DOMAIN_WHITELIST=${var.dns_name}|" .env
      sed -i "s|^DAYS_WARNING=.*|DAYS_WARNING=99999|" .env
      sed -i "s|^ALB_SA_KEY_B64=.*|ALB_SA_KEY_B64=$(base64 -w 0 sa-key.json)|" .env
      sed -i "s|^DNS_SA_KEY_B64=.*|DNS_SA_KEY_B64=$(base64 -w 0 sa-key.json)|" .env

      grep '^ALB_SA_KEY_B64=' .env | cut -d= -f2- | base64 -d >/dev/null && echo "ALB key OK"
      grep '^DNS_SA_KEY_B64=' .env | cut -d= -f2- | base64 -d >/dev/null && echo "DNS key OK"

      # DAYS_WARNING=99999 forces replacement regardless of bootstrap cert expiry.
      # Reset to 30 after the first successful run (Step 6).

    ── Step 4: On VM — build the Docker image ────────────────────

      docker build -t stackit-alb-cert-updater .

    ── Step 5: On VM — run certificate issuance ──────────────────

      docker run --rm \
        --env-file ~/stackit-acme-alb/.env \
        -v ~/stackit-acme-alb/letsencrypt_data:/etc/letsencrypt \
        stackit-alb-cert-updater

    ── Step 6: On VM — cron job for automatic renewal ────────────

      # Monthly on the 1st at 03:00 (LE certs valid 90 days, warning at 30)
      (crontab -l 2>/dev/null; echo "0 3 1 * * docker run --rm --env-file /home/debian/stackit-acme-alb/.env -v /home/debian/stackit-acme-alb/letsencrypt_data:/etc/letsencrypt stackit-alb-cert-updater >> /home/debian/stackit-acme-alb/renewal.log 2>&1") | crontab -

      crontab -l

  EOT
}
