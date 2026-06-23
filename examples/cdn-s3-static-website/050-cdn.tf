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

locals {
  bucket_endpoint   = "https://${stackit_objectstorage_bucket.website.name}.object.storage.${var.stackit_region}.onstackit.cloud"
  blocked_countries = length(var.cdn_blocked_countries) > 0 ? var.cdn_blocked_countries : null
}

resource "stackit_cdn_distribution" "website" {
  project_id = var.stackit_project_id
  config = {
    backend = {
      type       = "bucket"
      bucket_url = local.bucket_endpoint
      region     = var.stackit_region
      credentials = {
        access_key_id     = stackit_objectstorage_credential.cdn.access_key
        secret_access_key = stackit_objectstorage_credential.cdn.secret_access_key
      }
    }

    regions           = var.cdn_enabled_regions
    blocked_countries = local.blocked_countries

    optimizer = {
      enabled = false
    }

    // Work-in-Progress: redirects are not working with buckets
    /*redirects = {
      rules = [
        {
          description          = "Redirect / path to index"
          enabled              = true
          rule_match_condition = "ANY"
          status_code          = 301
          target_url           = "/"
          matchers = [
            {
              values                = ["/index"]
              value_match_condition = "ANY"
            }
          ]
        }
      ]
    }*/

    waf = {
      mode           = "ENABLED"
      type           = "FREE"
      paranoia_level = "L1"

      enabled_rule_collection_ids  = ["@builtin/crs/request"]
      log_only_rule_collection_ids = ["@builtin/crs/response"]


      allowed_http_versions         = ["HTTP/1.1", "HTTP/2"]
      allowed_http_methods          = ["GET", "HEAD"]
      allowed_request_content_types = ["text/html", "text/css", "text/plain", "application/javascript"]
    }
  }

  depends_on = [
    aws_s3_object.index_html,
    aws_s3_object.about_html,
    aws_s3_object.static_css,
  ]
}
