#! /bin/bash
: "${KCP_CLUSTER_CTX:=k3d-op-kcp}"
: "${SKR_CLUSTER_CTX:=k3d-op-skr}"


kubectl config use $SKR_CLUSTER_CTX

echo "apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample #change with your kyma name
  namespace: kyma-system
  labels:
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
    "operator.kyma-project.io/kyma-name": "kyma-sample"
type: Opaque
data:
  config: $(kubectl config view --raw --minify | sed 's/---//g' | base64)" > ./skr-secret.yaml

kubectl config use $KCP_CLUSTER_CTX

kubectl apply -f ./skr-secret.yaml

rm ./skr-secret.yaml
