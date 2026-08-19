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

set -e  # Exit immediately if a command exits with a non-zero status

# Help / Usage output
usage() {
    echo "Usage: $0 <upload|download> <filename>"
    echo "Examples:"
    echo "  $0 upload /path/to/local/file.txt"
    echo "  $0 download file.txt"
    exit 1
}

# Check for required arguments
if [ -z "$1" ] || [ -z "$2" ]; then
    usage
fi

ACTION="$1"
TARGET_FILE="$2"

# ==============================================================================
# CONFIGURATION
# ==============================================================================
export AWS_ACCESS_KEY_ID=""
export AWS_SECRET_ACCESS_KEY=""
export AWS_DEFAULT_REGION="eu01"

ENDPOINT="https://object.storage.eu01.onstackit.cloud"
BUCKET="s3://mybucket"
KEY_FILE="/root/ssec.key"

# ==============================================================================
# 1. GENERATE BINARY KEY (Only required once if the key file doesn't exist)
# ==============================================================================
if [ ! -f "$KEY_FILE" ]; then
    echo "[*] Generating new binary 32-byte AES key..."
    openssl rand 32 > "$KEY_FILE"
    chmod 600 "$KEY_FILE"
fi

# ==============================================================================
# 2. ACTION EXECUTION (UPLOAD OR DOWNLOAD)
# ==============================================================================
if [ "$ACTION" == "upload" ]; then
    # Check if the local file exists before uploading
    if [ ! -f "$TARGET_FILE" ]; then
        echo "Error: Local file '$TARGET_FILE' does not exist!"
        exit 1
    fi

    REMOTE_NAME=$(basename "$TARGET_FILE")

    echo "[*] Starting SSE-C upload of '$TARGET_FILE' to STACKIT..."
    aws --endpoint-url "$ENDPOINT" s3 cp "$TARGET_FILE" "$BUCKET/$REMOTE_NAME" \
      --sse-c AES256 \
      --sse-c-key "fileb://$KEY_FILE"

    echo "[+] Upload successful! '$REMOTE_NAME' is now stored encrypted in the bucket. 🚀"

elif [ "$ACTION" == "download" ]; then
    REMOTE_NAME=$(basename "$TARGET_FILE")
    LOCAL_DOWNLOAD_PATH="./${REMOTE_NAME}_downloaded"

    echo "[*] Starting SSE-C download of '$REMOTE_NAME' from STACKIT..."
    aws --endpoint-url "$ENDPOINT" s3 cp "$BUCKET/$REMOTE_NAME" "$LOCAL_DOWNLOAD_PATH" \
      --sse-c AES256 \
      --sse-c-key "fileb://$KEY_FILE"

    echo "[+] Download successful! Saved to '$LOCAL_DOWNLOAD_PATH' 🚀"

else
    echo "Error: Invalid action '$ACTION'."
    usage
fi
