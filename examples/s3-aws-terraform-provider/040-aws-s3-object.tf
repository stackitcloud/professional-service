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

resource "aws_s3_object" "example_file" {
  depends_on = [stackit_objectstorage_bucket.example]

  bucket  = stackit_objectstorage_bucket.example.name
  key     = "hello-world.txt"
  content = "Hello from STACKIT Object Storage managed via the AWS Terraform Provider!"
}
