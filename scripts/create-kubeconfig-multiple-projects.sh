#!/bin/bash

rm -rf ~/.kube/config
stackit auth login

projects=(
  "xxx-xxx-xxx-xxx"
)


for project in "${projects[@]}"; do
  stackit config set --project-id "$project"

  clusters_yaml=$(stackit ske cluster list -o yaml)

  cluster_names=$(echo "$clusters_yaml" | yq e '.[] | .name' -)

  for cluster_name in $cluster_names; do
    echo "Creating kubeconfig for cluster: $cluster_name"
    stackit ske kubeconfig create --expiration 60d "$cluster_name" -y
  done
done
