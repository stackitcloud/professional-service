#!/bin/bash

example01_wan_ip=$(terraform output -raw example01-wan-ip)
example02_wan_ip=$(terraform output -raw example02-wan-ip)
vip01_wan_ip=$(terraform output -raw vip01-wan-ip)

curl_ip() {
    local ip=$1
    echo "Performing curl on IP: $ip"
    curl http://$ip
    echo -e "\n" # For better readability in output
}

curl_ip $example01_wan_ip
curl_ip $example02_wan_ip
curl_ip $vip01_wan_ip

echo "Curl operations completed."
