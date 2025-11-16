#!/bin/bash

# Changing current directory to the root of the project
cd $(git rev-parse --show-toplevel)

# Export necessary environment variables
export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml
# Use the in-cluster registry name (without .localhost) so KLM can access it from inside the cluster
export CLUSTER_REGISTRY="k3d-kcp-registry:5000"
export LOCAL_IMG="localhost:5111/lifecycle-manager"
export CLUSTER_IMG="${CLUSTER_REGISTRY}/lifecycle-manager"
export TAG=$(date +%Y%m%d%H%M%S)

pushd config/manager
kustomize edit add patch --kind Deployment --patch $echo\
"- op: add
  path: /spec/template/spec/containers/0/args/-
  value: --oci-registry-host=http://${CLUSTER_REGISTRY}"
popd

make docker-build IMG=${LOCAL_IMG}:${TAG}
make docker-push IMG=${LOCAL_IMG}:${TAG}
make local-deploy-with-watcher IMG=${CLUSTER_IMG}:${TAG}

git restore config/manager/kustomization.yaml
