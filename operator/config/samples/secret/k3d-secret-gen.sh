#! /bin/bash

: "${KCP_CLUSTER_CTX:=k3d-op-kcp}"
: "${SKR_CLUSTER_CTX:=k3d-op-skr}"

kubectl config use $SKR_CLUSTER_CTX

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample #change with your kyma name
  namespace: kyma-system
  labels:
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
    "operator.kyma-project.io/kyma-name": "kyma-sample"
type: Opaque
data:
  config: $(cat /Users/D063994/SAPDevelop/go/kubeconfigs/skr.yaml | sed 's/---//g' | base64)"
EOF

kubectl config use $KCP_CLUSTER_CTX

kubectl apply -f ./skr-secret.yaml
rm ./skr-secret.yaml

