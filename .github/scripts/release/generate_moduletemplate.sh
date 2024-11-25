#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

MODULE_NAME=$1
RELEASE_VERSION=$2
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

cat module-config-for-e2e.yaml
modulectl create --config-file ./module-config-for-e2e.yaml --registry http://localhost:5111 --insecure
sed -i 's/localhost:5111/k3d-kcp-registry.localhost:5000/g' ./template.yaml
kubectl apply -f template.yaml

cat template.yaml
echo "ModuleTemplate created successfully"

rm -f module-config-for-e2e.yaml
rm -f template.yaml