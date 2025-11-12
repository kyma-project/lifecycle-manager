#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

MODULE_NAME=$1
RELEASE_VERSION=$2
INCLUDE_DEFAULT_CR=${3:-true}
MANDATORY=${4:-false}
DEPLOY_MODULETEMPLATE=${5:-true}
REQUIRES_DOWNTIME=${6:-false}

cat <<EOF > module-config-for-e2e.yaml
name: kyma-project.io/module/${MODULE_NAME}
version: ${RELEASE_VERSION}
security: sec-scanners-config.yaml
manifest: https://localhost:8080/template-operator.yaml
repository: https://github.com/kyma-project/template-operator
documentation: https://github.com/kyma-project/template-operator/blob/main/README.md
requiresDowntime: ${REQUIRES_DOWNTIME}
icons:
- name: module-icon
  link: https://github.com/kyma-project/template-operator/blob/main/docs/assets/logo.png
EOF

if [ "${INCLUDE_DEFAULT_CR}" == "true" ]; then
  cat <<EOF >> module-config-for-e2e.yaml
defaultCR: https://localhost:8080/config/samples/default-sample-cr.yaml
EOF
fi

if [ "${MANDATORY}" == "true" ]; then
  cat <<EOF >> module-config-for-e2e.yaml
mandatory: true
EOF
fi

# Replace the bdba list with the current module version
yq eval '.bdba = ["europe-docker.pkg.dev/kyma-project/prod/template-operator:'"${RELEASE_VERSION}"'"]' -i sec-scanners-config.yaml

MODULE_CONFIG="module-config-for-e2e.yaml"
REGISTRY_URL="http://localhost:5111/"
COMPONENT_CONSTRUCTOR_FILE="./component-constructor.yaml"
CTF_DIR="./component-ctf"
TEMPLATE_FILE="template.yaml"
MANIFEST_FILE="template-operator.yaml"
DEFAULT_CR_FILE="default-sample-cr.yaml"

cat "${MODULE_CONFIG}"

# Generate ModuleTemplate using modulectl
echo "Generating CTF with modulectl..."
modulectl create \
  --config-file "${MODULE_CONFIG}" \
  --disable-ocm-registry-push \
  --output-constructor-file "${COMPONENT_CONSTRUCTOR_FILE}"

# Transfer CTF to registry using ocm cli
echo "Transferring component version to registry using ocm cli..."
ocm add componentversions --create --file "${CTF_DIR}" --skip-digest-generation "${COMPONENT_CONSTRUCTOR_FILE}"
ocm transfer ctf --no-update "${CTF_DIR}" "${REGISTRY_URL}"

cat "${TEMPLATE_FILE}"
echo "ModuleTemplate created successfully"
yq -i '.metadata.namespace="kcp-system"' "${TEMPLATE_FILE}"

if [ "${DEPLOY_MODULETEMPLATE}" == "true" ]; then
  kubectl apply -f "${TEMPLATE_FILE}"
  rm -f "${TEMPLATE_FILE}"
fi

# Cleanup temporary files
rm -f "${MODULE_CONFIG}"
rm -f "${MANIFEST_FILE}"
rm -f "${DEFAULT_CR_FILE}"
rm -rf "${CTF_DIR}"
echo "Temporary files removed successfully"
