#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

# Required parameters
MODULE_NAME=$1
RELEASE_VERSION=$2
DEPLOYABLE_IMAGE_VERSION=$3  # template-operator version from versions.yaml (e.g., 1.0.4) - actual Docker image that exists

# Optional parameters with defaults
INCLUDE_DEFAULT_CR=${4:-true}
MANDATORY=${5:-false}
DEPLOY_MODULETEMPLATE=${6:-true}
REQUIRES_DOWNTIME=${7:-false}

# Validate required parameters
if [ -z "${MODULE_NAME}" ] || [ -z "${RELEASE_VERSION}" ] || [ -z "${DEPLOYABLE_IMAGE_VERSION}" ]; then
  echo "Error: MODULE_NAME, RELEASE_VERSION, and DEPLOYABLE_IMAGE_VERSION are required"
  echo "Usage: $0 MODULE_NAME RELEASE_VERSION DEPLOYABLE_IMAGE_VERSION [INCLUDE_DEFAULT_CR] [MANDATORY] [DEPLOY_MODULETEMPLATE] [REQUIRES_DOWNTIME]"
  echo ""
  echo "Required parameters:"
  echo "  MODULE_NAME                 - Name of the module (e.g., template-operator)"
  echo "  RELEASE_VERSION             - Version for OCM component descriptor (e.g., 1.1.1-e2e-test)"
  echo "  DEPLOYABLE_IMAGE_VERSION    - Actual image version that exists in registry (e.g., 1.0.4)"
  echo ""
  echo "Optional parameters (with defaults):"
  echo "  INCLUDE_DEFAULT_CR          - Include default CR in module (default: true)"
  echo "  MANDATORY                   - Mark module as mandatory (default: false)"
  echo "  DEPLOY_MODULETEMPLATE       - Deploy ModuleTemplate to cluster (default: true)"
  echo "  REQUIRES_DOWNTIME           - Module requires downtime for upgrades (default: false)"
  exit 1
fi

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

# Add moduleversion to `bdba` list in sec-scanners-config.yaml
yq eval '.bdba += ["europe-docker.pkg.dev/kyma-project/prod/template-operator:'"${RELEASE_VERSION}"'"]' -i sec-scanners-config.yaml

# Replace test version with deployable version in the manifest YAML
sed -i 's|europe-docker.pkg.dev/kyma-project/prod/template-operator:'"${RELEASE_VERSION}"'|europe-docker.pkg.dev/kyma-project/prod/template-operator:'"${DEPLOYABLE_IMAGE_VERSION}"'|g' template-operator.yaml

MODULE_CONFIG="module-config-for-e2e.yaml"
REGISTRY_URL="localhost:5111"
COMPONENT_CONSTRUCTOR_FILE="./component-constructor.yaml"
CTF_DIR="./component-ctf"
TEMPLATE_FILE="template.yaml"
MANIFEST_FILE="template-operator.yaml"
DEFAULT_CR_FILE="default-sample-cr.yaml"

echo "=== Module config for modulectl... ==="
cat "${MODULE_CONFIG}"

# Configure OCM to use insecure HTTP for local registry
OCM_CONFIG="$(dirname "$0")/ocm-config-local-registry.yaml"

# Generate ModuleTemplate using modulectl
echo "Generating CTF with modulectl..."
modulectl create \
  --config-file "${MODULE_CONFIG}" \
  --disable-ocm-registry-push \
  --output-constructor-file "${COMPONENT_CONSTRUCTOR_FILE}"

echo "=== Component Constructor file ==="
cat "${COMPONENT_CONSTRUCTOR_FILE}"

# Transfer CTF to registry using ocm cli
echo "Transferring component version to registry using ocm cli..."
ocm --config "${OCM_CONFIG}" add componentversions --create --file "${CTF_DIR}" --skip-digest-generation "${COMPONENT_CONSTRUCTOR_FILE}"
ocm --config "${OCM_CONFIG}" transfer ctf --overwrite --no-update "${CTF_DIR}" "http://${REGISTRY_URL}"

echo "[DEBUG] List Component Versions from local registry:"
echo ocm --config "${OCM_CONFIG}" get componentversions "http://${REGISTRY_URL}//kyma-project.io/module/${MODULE_NAME}"
ocm --config "${OCM_CONFIG}" get componentversions "http://${REGISTRY_URL}//kyma-project.io/module/${MODULE_NAME}"

echo "[DEBUG] Get ComponentDescriptor from local registry:"
echo ocm --config "${OCM_CONFIG}" get componentversion "http://${REGISTRY_URL}//kyma-project.io/module/${MODULE_NAME}:${RELEASE_VERSION}" -o yaml
ocm --config "${OCM_CONFIG}" get componentversion "http://${REGISTRY_URL}//kyma-project.io/module/${MODULE_NAME}:${RELEASE_VERSION}" -o yaml

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
