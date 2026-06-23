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

fail=0
for f; do
  case "$f" in
    modules/*|*/modules/*) continue ;;
  esac
  b="$(basename "$f")"
  if ! echo "$b" | grep -qE '^[0-9]{3}-'; then
    echo "ERROR: ${f}/${b} does not follow the 3-digit naming convention (e.g., 010-provider.tf, 020-variables.tf, 100-outputs.tf)"
    fail=1
  fi
done
exit "$fail"
