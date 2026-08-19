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


# Script to list (all) resources of a STACKIT project
# Ideally, you redirect the output into a markdown file.

set -euo pipefail

# Check if stackit cli is installed
if ! command -v stackit &> /dev/null; then
    echo "Error: stackit command not found"
    echo "Please install STACKIT CLI from:"
    echo "https://github.com/stackitcloud/stackit-cli/blob/main/INSTALLATION.md"
    exit 1
fi

# Check if stackit is properly authenticated; only login if the session is gone
if ! stackit auth get-access-token &> /dev/null; then
    echo "Session expired. Logging in..."
    stackit auth login || { echo "Error: stackit authentication failed."; exit 1; }
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "Error: 'jq' is not installed."
    exit 1
fi

# Function to display usage and handle errors
display_usage() {
    echo "Usage: $0 <project-id-1> <project-id-2> ..."
    echo "Example: $0 xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy >output.md"
    exit 1
}

# Function to get project information
get_project_name() {
    local project_id=$1

    local project_name=""
    project_name=$(stackit project describe --project-id "$project_id" --output-format json 2>/dev/null \
          | jq -r '.name // empty')
    # If the CLI call fails or .name is missing, fall back to the raw ID
    [[ -z "$project_name" ]] && project_name="$project_id"
    echo "## Project: $project_name ($project_id)"
    echo ""
}

# Function to get resource information
fetch_resources() {
    local service=$1
    local type=$2
    local label=$3
    local fields=${4:-}  # comma-separated list of fields to keep; empty = all

    # Build the CLI command - omit the type if it's empty
    if [[ -z "$type" ]]; then
        json_output=$(stackit "$service" list --project-id "$project_id" --output-format json)
    else
        json_output=$(stackit "$service" "$type" list --project-id "$project_id" --output-format json)
    fi

    local header
    local separation_line
    local data

    # Check whether json output is empty/null - indicating no resource of that service type found - and print it
    if [[ -z "$json_output" ]] || [[ "$json_output" == "[]" ]] || [[ "$json_output" == "null" ]]; then
        echo "**$label:**"
        echo "N/A"
        echo -e "\n"
        return
    fi

    # Narrow output to only the requested fields (preserving their order)
    if [[ -n "$fields" ]]; then
        json_output=$(jq --arg fields "$fields" '
            ($fields | split(",")) as $keep |
            map(. as $item | reduce $keep[] as $k ({}; . + {($k): $item[$k]}))
        ' <<< "$json_output")
    fi

    # Format the json output into a markdown table
    header=$(jq -r '.[0] | to_entries | map(.key) | join("\t")' <<< "$json_output" | sed 's/\t/|/g; s/^/|/; s/$/|/')
    separation_line=$(jq -r '.[0] | to_entries | map("|---") | join("")' <<< "$json_output")
    data=$(jq -r '.[] | to_entries | map(.value|tostring) | join("\t")' <<< "$json_output" | sed 's/\t/|/g; s/^/|/; s/$/|/')

    echo "**$label:**"
    echo "$header"
    echo "$separation_line"
    echo "$data"
    echo -e "\n"
}

get_project_resources() {

    # call function to get project name
    get_project_name "$project_id"

    # Format: service, type, label, fields (comma-separated; empty = all fields)
    services=("dns" "zone" "DNS Zones" "name,dnsName,type,visibility,state,recordCount,id"
              "git" "instance" "Git Instances" ""
              "load-balancer" "" "Load Balancers" ""
              "logme" "instance" "LogMe Instances" ""
              "mariadb" "instance" "MariaDB Instances" ""
              "mongodbflex" "instance" "MongoDB Flex Instances" ""
              "object-storage" "bucket" "Object Storage Buckets" ""
              "observability" "instance" "Observability Instances" ""
              "opensearch" "instance" "OpenSearch Instances" ""
              "postgresflex" "instance" "Postgres Flex Instances" ""
              "public-ip" "" "Public IPs" ""
              "rabbitmq" "instance" "RabbitMQ Instances" ""
              "redis" "instance" "Redis Instances" ""
              "secrets-manager" "instance" "Secrets Manager Instances" ""
              "service-account" "" "Service Accounts" ""
              "ske" "cluster" "SKE Clusters" "")

    for ((i=0; i<${#services[@]}; i+=4)); do
        fetch_resources "${services[i]}" "${services[i+1]}" "${services[i+2]}" "${services[i+3]}"
    done

    echo "last update: $(date +"%a, %d-%b-%Y %H:%M:%S %Z")"
}

# Main script
if [ $# -eq 0 ]; then
    echo "Error: No project IDs provided"
    display_usage
fi

# Process each project ID
for project_id in "$@"; do
    get_project_resources "$project_id"
done
