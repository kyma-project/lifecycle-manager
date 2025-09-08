#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

MODULE_NAME=$1
VERSION=$2

cat <<EOF > module-release-meta-mandatory.yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleReleaseMeta
metadata:
  name: ${MODULE_NAME}
  namespace: kcp-system
spec:
  moduleName: ${MODULE_NAME}
  ocmComponentName: kyma-project.io/module/${MODULE_NAME}
  mandatory:
    version: ${VERSION}
EOF

kubectl apply -f module-release-meta-mandatory.yaml

echo "Mandatory ModuleReleaseMeta created successfully"
rm -f module-release-meta-mandatory.yaml

kubectl get modulereleasemeta "${MODULE_NAME}" -n kcp-system -o yaml
kubectl get moduletemplate -n kcp-system
