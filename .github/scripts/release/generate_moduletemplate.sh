#!/usr/bin/env bash
# Usage: This script is used for generating ModuleTemplate with a specific version for e2e test, before running the release script, you have to provide the defaultCR and manifest files in the specified paths, and start the https server to serve the files.
set -o nounset
set -o errexit
set -E
set -o pipefail

RELEASE_VERSION=$1
MODULE_NAME=$2
INCLUDE_DEFAULT_CR=${3:-true}

cat <<EOF > module-config-for-e2e.yaml
name: kyma-project.io/module/template-operator
version: ${RELEASE_VERSION}
security: sec-scanners-config.yaml
manifest: https://localhost:8080/template-operator.yaml
repository: https://github.com/kyma-project/template-operator
documentation: https://github.com/kyma-project/template-operator/blob/main/README.md
icons:
- name: module-icon
  link: https://github.com/kyma-project/template-operator/blob/main/docs/assets/logo.png"
EOF

if [ "${INCLUDE_DEFAULT_CR}" == "true" ]; then
  cat <<EOF >> module-config-for-e2e.yaml
defaultCR: https://localhost:8080/config/samples/default-sample-cr.yaml
EOF
fi

modulectl create --config-file ./module-config-for-e2e.yaml --registry http://localhost:5111 --insecure
sed -i 's/localhost:5111/k3d-kcp-registry.localhost:5000/g' ./template.yaml
kubectl apply -f template.yaml

echo "ModuleTemplate created successfully"

cat <<EOF > module-release-meta.yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleReleaseMeta
metadata:
  name: ${MODULE_NAME}
  namespace: kcp-system
spec:
  channels:
  - channel: regular
    version: ${RELEASE_VERSION}
  moduleName: ${MODULE_NAME}
EOF
kubectl apply -f module-release-meta.yaml

echo "ModuleReleaseMeta created successfully"
