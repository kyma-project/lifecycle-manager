#!/bin/bash

# Exporting the path to the kubeconfig file
export KUBECONFIG=$HOME/.k3d/kcp-local.yaml

# Undeploying Kyma
kubectl -n kcp-system delete kyma kyma-sample
kubectl -n kcp-system delete secret kyma-sample

echo "[$(basename $0)] Kyma undeployed successfully"
