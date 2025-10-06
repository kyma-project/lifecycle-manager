#!/bin/bash

CERT_MANAGER_VERSION=$1

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v${CERT_MANAGER_VERSION}/cert-manager.yaml
  for deploy in cert-manager cert-manager-webhook cert-manager-cainjector; do
    kubectl rollout status deploy -n cert-manager "$deploy" --timeout=2m || exit 1
  done
