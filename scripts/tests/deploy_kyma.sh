#!/bin/bash

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample
  namespace: kcp-system
  labels:
    "operator.kyma-project.io/kyma-name": "kyma-sample"
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
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
