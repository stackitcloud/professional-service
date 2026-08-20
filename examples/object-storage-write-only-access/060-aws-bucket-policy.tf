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

resource "aws_s3_bucket_policy" "this" {
  bucket = stackit_objectstorage_bucket.this.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowWriteOnlyAccess"
        Effect    = "Allow"
        Principal = { AWS = stackit_objectstorage_credentials_group.write.urn }
        Action = [
          "s3:PutObject",
          "s3:AbortMultipartUpload",
          "s3:PutObjectRetention"
        ]
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}/*"
        ]
      }
    ]
  })
}
