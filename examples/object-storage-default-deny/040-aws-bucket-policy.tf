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

# By default, all Object Storage credentials in the same STACKIT project have
# full access ("default allow") to every bucket in that project.
# Because an explicit Deny in an S3 bucket policy always overrides the project's
# default Allow, we combine:
#   1. A default-deny statement using NotPrincipal to block all credentials not
#      explicitly listed for this bucket.
#   2. Role-specific Deny statements using NotAction so each listed credential
#      group can only perform its intended S3 actions.
resource "aws_s3_bucket_policy" "this" {
  bucket     = stackit_objectstorage_bucket.this.name
  depends_on = [stackit_objectstorage_credential.admin]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "DefaultDenyAllOtherCredentials"
        Effect = "Deny"
        NotPrincipal = {
          AWS = [
            stackit_objectstorage_credentials_group.admin.urn,
            stackit_objectstorage_credentials_group.write.urn,
            stackit_objectstorage_credentials_group.read.urn
          ]
        }
        Action = "s3:*"
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}",
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}/*"
        ]
      },
      {
        Sid       = "DenyBucketActionsForWriteCredentials"
        Effect    = "Deny"
        Principal = { AWS = stackit_objectstorage_credentials_group.write.urn }
        Action    = "s3:*"
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}"
        ]
      },
      {
        Sid       = "DenyNonWriteObjectActionsForWriteCredentials"
        Effect    = "Deny"
        Principal = { AWS = stackit_objectstorage_credentials_group.write.urn }
        NotAction = [
          "s3:PutObject",
          "s3:AbortMultipartUpload"
        ]
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}/*"
        ]
      },
      {
        Sid       = "DenyNonReadBucketActionsForReadCredentials"
        Effect    = "Deny"
        Principal = { AWS = stackit_objectstorage_credentials_group.read.urn }
        NotAction = [
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}"
        ]
      },
      {
        Sid       = "DenyNonReadObjectActionsForReadCredentials"
        Effect    = "Deny"
        Principal = { AWS = stackit_objectstorage_credentials_group.read.urn }
        NotAction = [
          "s3:GetObject"
        ]
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}/*"
        ]
      },
      {
        Sid       = "AllowAdminFullAccess"
        Effect    = "Allow"
        Principal = { AWS = stackit_objectstorage_credentials_group.admin.urn }
        Action    = "s3:*"
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}",
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}/*"
        ]
      },
      {
        Sid       = "AllowWriteOnlyAccess"
        Effect    = "Allow"
        Principal = { AWS = stackit_objectstorage_credentials_group.write.urn }
        Action = [
          "s3:PutObject",
          "s3:AbortMultipartUpload"
        ]
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}/*"
        ]
      },
      {
        Sid       = "AllowReadOnlyAccess"
        Effect    = "Allow"
        Principal = { AWS = stackit_objectstorage_credentials_group.read.urn }
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}",
          "arn:aws:s3:::${stackit_objectstorage_bucket.this.name}/*"
        ]
      }
    ]
  })
}
