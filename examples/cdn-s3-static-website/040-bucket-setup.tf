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

resource "aws_s3_bucket_policy" "public_read" {
  bucket = stackit_objectstorage_bucket.website.name
  policy = jsonencode({
    Statement = [
      {
        Sid       = "PublicReadGetObject"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "urn:sgws:s3:::${stackit_objectstorage_bucket.website.name}/*"
      }
    ]
  })
}

resource "aws_s3_object" "index_html" {
  bucket = stackit_objectstorage_bucket.website.name
  key    = "index.html"
  source = "${path.module}/files/index.html"

  content_type = "text/html"
  etag         = filemd5("${path.module}/files/index.html")
}

resource "aws_s3_object" "about_html" {
  bucket = stackit_objectstorage_bucket.website.name
  key    = "about.html"
  source = "${path.module}/files/about.html"

  content_type = "text/html"
  etag         = filemd5("${path.module}/files/about.html")
}

resource "aws_s3_object" "static_css" {
  bucket = stackit_objectstorage_bucket.website.name
  key    = "static/style.css"
  source = "${path.module}/files/static/style.css"

  content_type = "text/css"
  etag         = filemd5("${path.module}/files/static/style.css")
}
