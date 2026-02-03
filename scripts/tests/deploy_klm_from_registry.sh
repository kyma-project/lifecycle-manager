#!/usr/bin/env bash

set -o nounset
set -o errexit
set -E
set -o pipefail


if [ "$#" -ne 4 ]; then
  echo "Error: Exactly 2 arguments are required for both flags."
  echo "Usage: $0 --image-registry [dev../prod..] --image-tag latest"
  exit 1
fi

# Changing directory to the root of the project with git
cd "$(git rev-parse --show-toplevel)"

while [[ "$#" -gt 0 ]]; do
  case $1 in
    --image-registry) KLM_IMAGE_REGISTRY="$2"; shift ;;
    --image-tag) KLM_IMAGE_TAG="$2"; shift ;;
    *)
      echo "Unknown parameter passed: $1";
      echo "Usage: $0 --image-registry [dev../prod..] --image-tag latest";
      exit 1 ;;
  esac
  shift
done

maxRetry=5
for retry in $(seq 1 $maxRetry)
do
  if make local-deploy-with-watcher IMG=europe-docker.pkg.dev/kyma-project/${KLM_IMAGE_REGISTRY}/lifecycle-manager:${KLM_IMAGE_TAG}; then
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
