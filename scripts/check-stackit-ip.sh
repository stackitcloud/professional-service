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


# check-stackit-ip.sh: Checks whether an IP address, or every address a hostname
# resolves to, is in the STACKIT public IP ranges.
# Usage: ./check-stackit-ip.sh <ip-address-or-hostname>
# Example: ./check-stackit-ip.sh 45.129.40.1
# Example: ./check-stackit-ip.sh object.storage.eu01.onstackit.cloud

# The v2 endpoint lists the ranges of all regions; the deprecated v1 endpoint only lists eu01.
RANGES_URL="https://iaas.api.stackit.cloud/v2/networks/public-ip-ranges"

usage() {
    echo "Usage: $0 <ip-address-or-hostname-to-check>"
    echo "Example: $0 45.129.40.1"
    echo "Example: $0 object.storage.eu01.onstackit.cloud"
    echo
    echo "For a hostname, the script checks the addresses DNS returns now."
    echo "STACKIT publishes ranges, not addresses per host, so use the ranges for firewall rules."
}

require_commands() {
    for cmd in "$@"; do
        if ! command -v "$cmd" &> /dev/null; then
            echo "Error: Required command '$cmd' is not installed." >&2
            echo "Please install it and ensure it's in your PATH." >&2
            exit 1
        fi
    done
}

# Loads the published ranges into RANGES, one "<cidr> <region>" per line.
fetch_ranges() {
    echo "Fetching STACKIT IP ranges..." >&2
    RANGES=$(stackit curl "$RANGES_URL" | jq -r '.items[] | "\(.cidr) \(.region)"')
    if [ -z "$RANGES" ]; then
        echo "Error: Failed to fetch or parse IP ranges from STACKIT API." >&2
        exit 1
    fi
}

# Prints "<cidr> <region>" of the published range that contains the address.
range_of() {
    local range region
    while read -r range region; do
        if [ -n "$range" ] && grepcidr -s "$range" <<< "$1" > /dev/null; then
            echo "$range $region"
            return 0
        fi
    done <<< "$RANGES"
    return 1
}

is_ip_address() {
    [[ "$1" == *:* || "$1" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]]
}

check_ip_address() {
    local ip="$1" match
    require_commands stackit jq grepcidr
    fetch_ranges
    if match=$(range_of "$ip"); then
        # Earlier versions let grepcidr print the matching address; keep that line.
        echo "$ip"
        echo "Found: $ip is in the range ${match% *} (region ${match#* })"
        exit 0
    fi
    echo "Not found: $ip is not in any of the STACKIT ranges."
    exit 1
}

# Prints the answer section of a dig query; exits if no DNS server replied.
dig_answer() {
    local answer
    if ! answer=$(dig +noall +answer "$1" "$2"); then
        echo "Error: The DNS query for $1 ($2) failed." >&2
        exit 1
    fi
    echo "$answer"
}

check_hostname() {
    local host="$1" a_answer aaaa_answer addresses chain status ip match all_found=1
    require_commands dig

    # Resolve first, so an unknown name fails before the API call.
    a_answer=$(dig_answer "$host" A) || exit 1
    aaaa_answer=$(dig_answer "$host" AAAA) || exit 1
    addresses=$(awk '$4 == "A" || $4 == "AAAA" { print $5 }' <<< "$a_answer"$'\n'"$aaaa_answer")
    if [ -z "$addresses" ]; then
        status=$(dig +noall +comments "$host" A | sed -n 's/.*status: \([A-Z]*\).*/\1/p')
        echo "Error: $host has no A or AAAA records (DNS status: ${status:-unknown})." >&2
        exit 1
    fi

    chain=$(awk '$4 == "CNAME" { sub(/\.$/, "", $5); print $5 }' <<< "$a_answer")
    if [ -z "$chain" ]; then
        chain=$(awk '$4 == "CNAME" { sub(/\.$/, "", $5); print $5 }' <<< "$aaaa_answer")
    fi

    require_commands stackit jq grepcidr
    fetch_ranges

    if [ -n "$chain" ]; then
        echo "CNAME chain: $host -> ${chain//$'\n'/ -> }"
    fi
    echo "$host resolves now to:"
    while read -r ip; do
        if match=$(range_of "$ip"); then
            printf '  %-16s in the range %s (region %s)\n' "$ip" "${match% *}" "${match#* }"
            continue
        fi
        all_found=0
        if [[ "$ip" == *:* && "$RANGES" != *:* ]]; then
            printf '  %-16s not checked, the API publishes no IPv6 ranges\n' "$ip"
        else
            printf '  %-16s not in the published ranges\n' "$ip"
        fi
    done <<< "$addresses"

    if [ "$all_found" -eq 1 ]; then
        echo "Found: every address of $host is in the STACKIT ranges."
        exit 0
    fi
    echo "Not found: not every address of $host is in the STACKIT ranges."
    exit 1
}

case "$1" in
    "")
        usage
        exit 1
        ;;
    -h | --help)
        usage
        exit 0
        ;;
esac

if is_ip_address "$1"; then
    check_ip_address "$1"
else
    check_hostname "$1"
fi
