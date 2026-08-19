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


export STACKIT_CLI_MIN_MAJOR=0
export STACKIT_CLI_MIN_MINOR=59
export STACKIT_CLI_MIN_PATCH=0

# Script to show SKE cluster versions and nodepool flatcar versions

set -euo pipefail

# Check if stackit command is available
if ! command -v stackit &> /dev/null; then
    echo "Error: stackit command not found"
    echo "Please install STACKIT CLI from:"
    echo "https://github.com/stackitcloud/stackit-cli/blob/main/INSTALLATION.md"
    exit 1
fi

# Check if stackit-cli is supported
stackit -v |awk -v maj="$STACKIT_CLI_MIN_MAJOR" -v min="$STACKIT_CLI_MIN_MINOR" -v pat="$STACKIT_CLI_MIN_PATCH" '/Version:/ { { split($2, a, ".");
current_maj = a[1];
current_min = a[2];
current_pat = a[3];
if (current_maj > maj || (current_maj == maj && current_min > min) || (current_maj == maj && current_min == min && current_pat >= pat)) {
    print "✅ Version supported (" current_maj "." current_min "." current_pat ")";
        exit 0;
    } else {
        print "❌ STACKIT CLI version not supported. Need at least " maj "." min "." pat;
        exit 1;
        }
    }
}'

# Define project IDs (space-separated)
projectid="xxxxxx yyyyy"

# Collect all clusters from all projects
all_clusters="[]"

echo "Fetching SKE cluster information..."
for pid in $projectid; do
    echo "Fetching clusters for project ID: $pid"
    clusters=$(stackit ske cluster list -o json --project-id "$pid")

    # Merge with existing clusters
    all_clusters=$(echo "$all_clusters" "$clusters" | jq -s '.[0] + .[1]')
done

# Print header
printf "\n%-20s %-30s %-20s %-30s %-15s %-40s\n" "CLUSTER NAME" "SKE VERSION" "NODEPOOL" "FLATCAR VERSION" "MACHINE TYPE" "PROJECT ID"
printf "%-20s %-30s %-20s %-30s %-15s %-40s\n" "------------" "-----------" "--------" "---------------" "------------" "----------"

# Parse JSON and display information
for pid in $projectid; do
    clusters=$(stackit ske cluster list -o json --project-id "$pid")

    echo "$clusters" | jq -r --arg pid "$pid" '.[] |
      .name as $cluster_name |
      .kubernetes.version as $k8s_version |
      .nodepools[] |
      [$cluster_name, $k8s_version, .name, .machine.image.version, .machine.type, $pid] |
      @tsv' | while IFS=$'\t' read -r cluster ske_version nodepool flatcar_version machine_type project; do

        k8s_version_desc=$(stackit ske options kubernetes-versions -o json | jq -r --arg VERSION $ske_version '.kubernetesVersions[] | select(.version == $VERSION) | if .state == "deprecated" then "exp. "+.expirationDate[0:10] else "supported" end ')
        k8s_version_state=$(stackit ske options kubernetes-versions -o json | jq -r --arg VERSION $ske_version '.kubernetesVersions[] | select(.version == $VERSION).state')

        flatcar_version_desc=$(stackit ske options machine-images -o json | jq -r --arg VERSION $flatcar_version '.machineImages[] | select(.name == "flatcar") | .versions[] | select(.version == $VERSION) | if .state == "deprecated" then "exp. "+.expirationDate[0:10] else "supported" end ' )
        flatcar_version_state=$(stackit ske options machine-images -o json | jq -r --arg VERSION $flatcar_version '.machineImages[] | select(.name == "flatcar") | .versions[] | select(.version == $VERSION).state' )

        GREEN='\033[0;32m'
        RED='\033[0;31m'
        NC='\033[0m'

        if [[ "$k8s_version_state" == "deprecated" || "$flatcar_version_state" == "deprecated" ]]; then
            ROW_COLOR=$RED
        else
            ROW_COLOR=$GREEN
        fi

        printf "${ROW_COLOR}%-20s %-30s  %-20s %-30s %-15s %-40s${NC}\n" "$cluster" "$ske_version ($k8s_version_desc)" "$nodepool" "$flatcar_version ($flatcar_version_desc) " "$machine_type" "$project"
    done
done

echo ""
echo "Summary:"
echo "$all_clusters" | jq -r 'length | "Total clusters: \(.)"'
