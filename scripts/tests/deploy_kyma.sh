#!/bin/bash

print_help() {
  echo "Usage: $0 [localhost | host.k3d.internal]"
  echo "localhost: Use this option when running KLM locally on the machine"
  echo "host.k3d.internal: Use this option when deploying KLM onto a cluster"
}

if [ "$#" -ne 1 ]; then
  print_help
  exit 1
fi

SKR_HOST=$1
if [ "$SKR_HOST" != "localhost" ] && [ "$SKR_HOST" != "host.k3d.internal" ]; then
  echo "Invalid option: $SKR_HOST"
  print_help
  exit 1
fi

# Exporting the path to the kubeconfig file
export KUBECONFIG=$HOME/.k3d/kcp-local.yaml
export SKR_HOST

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample
  namespace: kcp-system
  labels:
    "operator.kyma-project.io/kyma-name": "kyma-sample"
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
    "kyma-project.io/runtime-id": "kyma-sample"
data:
  config: $(k3d kubeconfig get skr | sed "s/0\.0\.0\.0/${SKR_HOST}/" | base64 | tr -d '\n')
---
apiVersion: operator.kyma-project.io/v1beta2
kind: Kyma
metadata:
  annotations:
    skr-domain: "example.domain.com"
  name: kyma-sample
  namespace: kcp-system
spec:
  channel: regular
EOF

if [ $? -ne 0 ]; then
  echo "[$(basename "$0")] Kyma deployment failed"
  exit 1
fi

echo "[$(basename "$0")] Kyma deployed successfully"
