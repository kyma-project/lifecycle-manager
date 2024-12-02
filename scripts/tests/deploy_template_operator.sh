#!/bin/bash

# Changing current directory to the root of the repository
cd "$(git rev-parse --show-toplevel)"

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <arg1>"
  exit 1
fi

# Exporting the path to the kubeconfig file
export KUBECONFIG=$HOME/.k3d/kcp-local.yaml

# Deploying the template-operator
kubectl apply -f tests/e2e/moduletemplate/$1

echo "[$0] Template operator deployed successfully"
