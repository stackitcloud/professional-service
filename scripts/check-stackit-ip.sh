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


# check-ip: Checks if a given IP address is in the STACKIT public IP ranges.
# Usage: ./check-ip <ip-address>
# Example: ./check-ip 45.129.40.1

# Check if an IP address was provided as an argument
if [ -z "$1" ]; then
    echo "Usage: $0 <ip-address-to-check>"
    echo "Example: $0 45.129.40.1"
    exit 1
fi

IP_TO_CHECK="$1"

# Check for required commands.
# We need 'stackit', 'jq', and 'grepcidr'.
for cmd in stackit jq grepcidr; do
    if ! command -v $cmd &> /dev/null; then
        echo "Error: Required command '$cmd' is not installed." >&2
        echo "Please install it and ensure it's in your PATH." >&2
        exit 1
    fi
done

echo "Fetching STACKIT IP ranges..." >&2

FOUND=0
RANGES_CHECKED=0

while read -r RANGE; do
    # Skip empty lines, just in case
    if [ -z "$RANGE" ]; then
        continue
    fi

    RANGES_CHECKED=$((RANGES_CHECKED + 1))

    if echo "$IP_TO_CHECK" | grepcidr -s "$RANGE"; then
        echo "Found: $IP_TO_CHECK is in the range $RANGE"
        FOUND=1
        break
    fi
done < <(stackit curl "https://iaas.api.eu01.stackit.cloud/v1/networks/public-ip-ranges" | jq -r '.items[].cidr')

# Check if we processed any ranges at all
if [ $RANGES_CHECKED -eq 0 ]; then
    echo "Error: Failed to fetch or parse IP ranges from STACKIT API." >&2
    exit 1
fi

# Check the flag after the loop
if [ $FOUND -eq 0 ]; then
    # If the loop finished without finding anything, print a "not found" message
    echo "Not found: $IP_TO_CHECK is not in any of the STACKIT ranges."
    exit 1
fi

exit 0
