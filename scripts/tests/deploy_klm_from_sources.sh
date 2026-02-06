#!/usr/bin/env bash

set -o nounset
set -o errexit
set -E
set -o pipefail

# Changing current directory to the root of the project
cd $(git rev-parse --show-toplevel)

# Export necessary environment variables
export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml
export LOCAL_IMG="localhost:5111/lifecycle-manager"
export CLUSTER_IMG="k3d-kcp-registry.localhost:5000/lifecycle-manager"
export TAG=$(date +%Y%m%d%H%M%S)

make docker-build IMG=${LOCAL_IMG}:${TAG}
make docker-push IMG=${LOCAL_IMG}:${TAG}

maxRetry=5
for retry in $(seq 1 $maxRetry)
do
  if make local-deploy-with-watcher IMG=${CLUSTER_IMG}:${TAG}; then
    set +e
    kubectl wait pods -n kcp-system -l app.kubernetes.io/name=lifecycle-manager --for condition=Ready --timeout=20s
    status=$?
    set -e
    if [[ $status -ne 0 ]]; then
      echo "KLM pods are not ready yet, will retry deployment"
      continue
    fi
    echo "KLM deployed successfully"
    exit 0
  elif [[ $retry -lt $maxRetry ]]; then
    echo "Deploy encountered some error, will retry after 20 seconds"
    sleep 20
  else
    echo "KLM deployment failed"
    exit 1
  fi
done
