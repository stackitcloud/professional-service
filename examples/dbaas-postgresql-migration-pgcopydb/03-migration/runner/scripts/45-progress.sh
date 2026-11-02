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

# Watch a running 40-clone from a second terminal: what pgcopydb writes to the
# target right now, and whether every worker connection is encrypted.
# Usage:  sudo ./45-progress.sh <database> [interval seconds]    (Ctrl-C to stop)
source "$(dirname "$0")/lib.sh"
need_db "$@"
interval="${2:-5}"

while true; do
    printf '\033[2J\033[H'
    echo "$(date -u +%H:%M:%S) UTC  $TARGET_HOST/$DB  (every ${interval}s, Ctrl-C to stop)"
    # Aligned output with headers, unlike src_sql/dst_sql: this is for reading.
    psql -X -q "$(dst_uri "$DB")" -f "$SQL_DIR/progress.sql"
    sleep "$interval"
done
