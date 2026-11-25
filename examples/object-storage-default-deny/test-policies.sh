#!/usr/bin/env bash
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

# Verifies the S3 default-deny and role-specific bucket policy statements
# using s5cmd across all four credential groups in the same STACKIT project:
#   1. unprivileged (denied) -> PutObject, ListBucket, GetObject, DeleteObject all denied
#   2. write-only   (write)  -> PutObject allowed; ListBucket, GetObject, DeleteObject denied
#   3. read-only    (read)   -> ListBucket, GetObject allowed; PutObject, DeleteObject denied
#   4. admin        (admin)  -> PutObject, ListBucket, GetObject, DeleteObject all allowed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if ! command -v s5cmd >/dev/null 2>&1; then
  echo "Error: 's5cmd' is required but not installed." >&2
  exit 1
fi

if command -v tofu >/dev/null 2>&1; then
  TF_BIN="tofu"
elif command -v terraform >/dev/null 2>&1; then
  TF_BIN="terraform"
else
  echo "Error: neither 'tofu' nor 'terraform' was found in PATH." >&2
  exit 1
fi

echo "[*] Reading outputs using $TF_BIN..."
BUCKET="$($TF_BIN output -raw bucket_name)"
ENDPOINT="$($TF_BIN output -raw s3_endpoint)"

ADMIN_ACCESS_KEY="$($TF_BIN output -raw admin_access_key)"
ADMIN_SECRET_KEY="$($TF_BIN output -raw admin_secret_access_key)"

WRITE_ACCESS_KEY="$($TF_BIN output -raw write_access_key)"
WRITE_SECRET_KEY="$($TF_BIN output -raw write_secret_access_key)"

READ_ACCESS_KEY="$($TF_BIN output -raw read_access_key)"
READ_SECRET_KEY="$($TF_BIN output -raw read_secret_access_key)"

DENIED_ACCESS_KEY="$($TF_BIN output -raw denied_access_key)"
DENIED_SECRET_KEY="$($TF_BIN output -raw denied_secret_access_key)"

export AWS_REGION="eu01"
export AWS_DEFAULT_REGION="eu01"

TMP_FILE="$(mktemp)"
echo "stackit-s3-default-deny-test-payload" >"$TMP_FILE"

WRITE_OBJECT_KEY="test-write-only.txt"
ADMIN_OBJECT_KEY="test-admin.txt"

PASS_COUNT=0
FAIL_COUNT=0

cleanup() {
  rm -f "$TMP_FILE"
  AWS_ACCESS_KEY_ID="$ADMIN_ACCESS_KEY" \
    AWS_SECRET_ACCESS_KEY="$ADMIN_SECRET_KEY" \
    s5cmd --endpoint-url "$ENDPOINT" rm "s3://$BUCKET/$WRITE_OBJECT_KEY" "s3://$BUCKET/$ADMIN_OBJECT_KEY" >/dev/null 2>&1 || true
}
trap cleanup EXIT

run_s5cmd() {
  local access_key="$1"
  local secret_key="$2"
  shift 2
  AWS_ACCESS_KEY_ID="$access_key" \
    AWS_SECRET_ACCESS_KEY="$secret_key" \
    s5cmd -r 0 --endpoint-url "$ENDPOINT" "$@"
}

expect_allow() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "  [PASS] ALLOW as expected: $description"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  [FAIL] Expected ALLOW but got DENY/error: $description"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

expect_deny() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "  [FAIL] Expected DENY but operation succeeded: $description"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  else
    echo "  [PASS] DENY as expected:  $description"
    PASS_COUNT=$((PASS_COUNT + 1))
  fi
}

echo
echo "=== 1. Testing Write-Only Credentials (write-only-credentials) ==="
expect_allow "write-only: PutObject (cp)" \
  run_s5cmd "$WRITE_ACCESS_KEY" "$WRITE_SECRET_KEY" cp "$TMP_FILE" "s3://$BUCKET/$WRITE_OBJECT_KEY"
expect_deny "write-only: ListBucket (ls)" \
  run_s5cmd "$WRITE_ACCESS_KEY" "$WRITE_SECRET_KEY" ls "s3://$BUCKET/"
expect_deny "write-only: GetObject (cat)" \
  run_s5cmd "$WRITE_ACCESS_KEY" "$WRITE_SECRET_KEY" cat "s3://$BUCKET/$WRITE_OBJECT_KEY"
expect_deny "write-only: DeleteObject (rm)" \
  run_s5cmd "$WRITE_ACCESS_KEY" "$WRITE_SECRET_KEY" rm "s3://$BUCKET/$WRITE_OBJECT_KEY"

echo
echo "=== 2. Testing Read-Only Credentials (read-only-credentials) ==="
expect_allow "read-only: ListBucket (ls)" \
  run_s5cmd "$READ_ACCESS_KEY" "$READ_SECRET_KEY" ls "s3://$BUCKET/"
expect_allow "read-only: GetObject (cat)" \
  run_s5cmd "$READ_ACCESS_KEY" "$READ_SECRET_KEY" cat "s3://$BUCKET/$WRITE_OBJECT_KEY"
expect_deny "read-only: PutObject (cp)" \
  run_s5cmd "$READ_ACCESS_KEY" "$READ_SECRET_KEY" cp "$TMP_FILE" "s3://$BUCKET/read-attempt.txt"
expect_deny "read-only: DeleteObject (rm)" \
  run_s5cmd "$READ_ACCESS_KEY" "$READ_SECRET_KEY" rm "s3://$BUCKET/$WRITE_OBJECT_KEY"

echo
echo "=== 3. Testing Unprivileged Credentials (unprivileged-credentials / Default Deny) ==="
expect_deny "unprivileged: ListBucket (ls)" \
  run_s5cmd "$DENIED_ACCESS_KEY" "$DENIED_SECRET_KEY" ls "s3://$BUCKET/"
expect_deny "unprivileged: GetObject (cat)" \
  run_s5cmd "$DENIED_ACCESS_KEY" "$DENIED_SECRET_KEY" cat "s3://$BUCKET/$WRITE_OBJECT_KEY"
expect_deny "unprivileged: PutObject (cp)" \
  run_s5cmd "$DENIED_ACCESS_KEY" "$DENIED_SECRET_KEY" cp "$TMP_FILE" "s3://$BUCKET/denied-attempt.txt"
expect_deny "unprivileged: DeleteObject (rm)" \
  run_s5cmd "$DENIED_ACCESS_KEY" "$DENIED_SECRET_KEY" rm "s3://$BUCKET/$WRITE_OBJECT_KEY"

echo
echo "=== 4. Testing Admin Credentials (admin-credentials) ==="
expect_allow "admin: PutObject (cp)" \
  run_s5cmd "$ADMIN_ACCESS_KEY" "$ADMIN_SECRET_KEY" cp "$TMP_FILE" "s3://$BUCKET/$ADMIN_OBJECT_KEY"
expect_allow "admin: ListBucket (ls)" \
  run_s5cmd "$ADMIN_ACCESS_KEY" "$ADMIN_SECRET_KEY" ls "s3://$BUCKET/"
expect_allow "admin: GetObject (cat)" \
  run_s5cmd "$ADMIN_ACCESS_KEY" "$ADMIN_SECRET_KEY" cat "s3://$BUCKET/$ADMIN_OBJECT_KEY"
expect_allow "admin: DeleteObject (rm)" \
  run_s5cmd "$ADMIN_ACCESS_KEY" "$ADMIN_SECRET_KEY" rm "s3://$BUCKET/$WRITE_OBJECT_KEY" "s3://$BUCKET/$ADMIN_OBJECT_KEY"

echo
echo "=================================================================="
echo "Results: $PASS_COUNT passed, $FAIL_COUNT failed"
echo "=================================================================="

if [ "$FAIL_COUNT" -ne 0 ]; then
  exit 1
fi
