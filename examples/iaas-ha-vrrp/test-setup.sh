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


active01_wan_ip=$(terraform output -raw active01-wan-ip)
passive02_wan_ip=$(terraform output -raw passive02-wan-ip)
vip01_wan_ip=$(terraform output -raw vip01-wan-ip)

curl_ip() {
    local ip=$1
    echo "Performing curl on IP: $ip"
    curl http://$ip
    echo -e "\n" # For better readability in output
}

curl_ip $active01_wan_ip
curl_ip $passive02_wan_ip
curl_ip $vip01_wan_ip

echo "Curl operations completed."
