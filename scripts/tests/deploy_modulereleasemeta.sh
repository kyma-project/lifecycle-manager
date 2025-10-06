#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

MODULE_NAME=$1
shift 1
CHANNELS=("$@")
cat <<EOF > module-release-meta.yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleReleaseMeta
metadata:
  name: ${MODULE_NAME}
  namespace: kcp-system
spec:
  moduleName: ${MODULE_NAME}
  ocmComponentName: kyma-project.io/module/${MODULE_NAME}
  channels:
EOF

for CHANNEL in "${CHANNELS[@]}"; do
  IFS=':' read -r CHANNEL_NAME CHANNEL_VERSION <<< "${CHANNEL}"
  cat <<EOF >> module-release-meta.yaml
  - channel: ${CHANNEL_NAME}
    version: ${CHANNEL_VERSION}
EOF
done
kubectl apply -f module-release-meta.yaml

echo "ModuleReleaseMeta created successfully"
rm -f module-release-meta.yaml

kubectl get modulereleasemeta "${MODULE_NAME}" -n kcp-system -o yaml
kubectl get moduletemplate -n kcp-system
