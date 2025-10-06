#!/bin/bash

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

export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml
IMG_REGISTRY_HOST="europe-docker.pkg.dev/kyma-project"
IMG_NAME="lifecycle-manager"
make local-deploy-with-watcher IMG=${IMG_REGISTRY_HOST}/${KLM_IMAGE_REGISTRY}/${IMG_NAME}:${KLM_IMAGE_TAG}

echo "[$(basename $0)] KLM deployed successfully from registry"
