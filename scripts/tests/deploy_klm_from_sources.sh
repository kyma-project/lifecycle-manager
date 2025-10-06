#!/bin/bash

# Changing current directory to the root of the project
cd $(git rev-parse --show-toplevel)

# Export necessary environment variables
export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml
export LOCAL_IMG="localhost:5111/lifecycle-manager"
export CLUSTER_IMG="k3d-kcp-registry.localhost:5000/lifecycle-manager"
export TAG=$(date +%Y%m%d%H%M%S)

make docker-build IMG=${LOCAL_IMG}:${TAG}
make docker-push IMG=${LOCAL_IMG}:${TAG}
make local-deploy-with-watcher IMG=${CLUSTER_IMG}:${TAG}
