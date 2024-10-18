#!/bin/bash

echo "Enter the KLM Image Registry (dev/prod):"
read KLM_IMAGE_REGISTRY

echo "Enter the KLM Image Tag (e.g., latest):"
read KLM_IMAGE_TAG

export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml
make local-deploy-with-watcher IMG=${IMG_REGISTRY_HOST}/${KLM_IMAGE_REGISTRY}/${IMG_NAME}:${KLM_IMAGE_TAG}