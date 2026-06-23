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

# Primary DNS zone — used for ACME DNS-01 challenge delegation
# After apply: delegate this zone at your registrar using the output nameservers
resource "stackit_dns_zone" "workshop" {
  project_id    = stackit_resourcemanager_project.showcase.project_id
  name          = var.dns_zone_name
  dns_name      = var.dns_name
  contact_email = var.dns_contact_email
  type          = "primary"
  default_ttl   = 300

  description = "Workshop zone for ALB + Certbot ACME DNS-01 challenge"
}

# A-record pointing the zone apex to the ALB public IP
resource "stackit_dns_record_set" "alb_a" {
  project_id = stackit_resourcemanager_project.showcase.project_id
  zone_id    = stackit_dns_zone.workshop.zone_id
  name       = var.dns_name
  type       = "A"
  ttl        = 300
  records    = [stackit_public_ip.alb.ip]
}
