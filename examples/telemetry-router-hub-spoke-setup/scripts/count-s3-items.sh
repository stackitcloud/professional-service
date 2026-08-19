#!/bin/bash
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

# This script counts objects in a STACKIT S3 bucket.
# It automatically retrieves configuration and credentials from the terraform state.
# Requirements: aws cli, terraform, jq installed

set -e

echo "[*] Retrieving S3 configuration from Terraform state..."

# Check if terraform state is available in current directory
if ! terraform output -json > /dev/null 2>&1; then
    echo "Error: Could not read terraform output. Make sure you are in the terraform directory and have run 'terraform apply' first."
    exit 1
fi

# Retrieve values from terraform output
ACCESS_KEY=$(terraform output -raw s3_access_key)
SECRET_KEY=$(terraform output -raw s3_secret_key)
ENDPOINT=$(terraform output -raw s3_endpoint)
BUCKET=$(terraform output -raw s3_archive_bucket)

# Configure AWS CLI environment
export AWS_ACCESS_KEY_ID="$ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$SECRET_KEY"
export AWS_DEFAULT_REGION="eu01"

# Count objects using aws cli
# We use xargs to trim whitespace from wc output
COUNT=$(aws --endpoint-url "$ENDPOINT" s3 ls "s3://$BUCKET" --recursive | grep -v "^$" | wc -l | xargs)

echo "[+] Current object count in s3://$BUCKET: $COUNT"
