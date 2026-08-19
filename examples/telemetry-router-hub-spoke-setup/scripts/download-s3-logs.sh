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

# This script downloads all objects from the STACKIT S3 archive bucket to a local directory,
# extracts compressed log files (.gz), and beautifies JSON content.
#
# It automatically retrieves configuration and credentials from the terraform state.
# Requirements: aws cli, terraform, jq, gunzip installed

set -e

echo "[*] Retrieving S3 configuration from Terraform state..."

# Check if terraform state is available (script should be run from the terraform directory)
if ! terraform output -json > /dev/null 2>&1; then
    echo "Error: Could not read terraform output. Make sure you have run 'terraform apply' first and are calling this script from the terraform directory."
    exit 1
fi

# Retrieve values from terraform output
ACCESS_KEY=$(terraform output -raw s3_access_key)
SECRET_KEY=$(terraform output -raw s3_secret_key)
ENDPOINT=$(terraform output -raw s3_endpoint)
BUCKET=$(terraform output -raw s3_archive_bucket)

# Get script directory to create downloads folder there
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOWNLOAD_DIR="$SCRIPT_DIR/downloads"

# Create download directory
mkdir -p "$DOWNLOAD_DIR"

# Configure AWS CLI environment
export AWS_ACCESS_KEY_ID="$ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$SECRET_KEY"
export AWS_DEFAULT_REGION="eu01"

echo "[*] Starting download from s3://$BUCKET to $DOWNLOAD_DIR..."
echo "[*] Endpoint: $ENDPOINT"

# Use sync to download all files efficiently
aws --endpoint-url "$ENDPOINT" s3 sync "s3://$BUCKET" "$DOWNLOAD_DIR"

echo "[*] Extracting compressed log files (.gz)..."
# Find all .gz files in the download directory and unzip them
find "$DOWNLOAD_DIR" -name "*.gz" -exec gunzip -f {} +

echo "[*] Beautifying JSON log files..."
# Find all files (now uncompressed) and try to beautify them with jq if they contain JSON
# We use a temporary file to perform in-place beautification
find "$DOWNLOAD_DIR" -type f ! -name "*.gz" | while read -r file; do
    if jq . "$file" > "$file.tmp" 2>/dev/null; then
        mv "$file.tmp" "$file"
    else
        rm -f "$file.tmp"
    fi
done

echo "[+] Download, extraction, and beautification complete! Files are located in $DOWNLOAD_DIR"
