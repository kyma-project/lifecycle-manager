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

# Add moduleversion to `bdba` list in sec-scanners-config.yaml
yq eval '.bdba += ["europe-docker.pkg.dev/kyma-project/prod/template-operator:'"${RELEASE_VERSION}"'"]' -i sec-scanners-config.yaml

cat module-config-for-e2e.yaml
modulectl create --config-file ./module-config-for-e2e.yaml --registry http://localhost:5111 --insecure

# Configure OCM to use insecure HTTP for local registry
OCM_CONFIG="$(dirname "$0")/ocm-config-local-registry.yaml"
REGISTRY_URL="localhost:5111"

echo "[DEBUG] List Component Versions from local registry:"
echo ocm --config "${OCM_CONFIG}" get componentversions "http://${REGISTRY_URL}//kyma-project.io/module/template-operator" -o yaml
ocm --config "${OCM_CONFIG}" get componentversions "http://${REGISTRY_URL}//kyma-project.io/module/template-operator" -o yaml

echo "[DEBUG] Get ComponentDescriptor from local registry:"
echo ocm --config "${OCM_CONFIG}" get componentversion "http://${REGISTRY_URL}//kyma-project.io/module/template-operator:${RELEASE_VERSION}" -o yaml
ocm --config "${OCM_CONFIG}" get componentversion "http://${REGISTRY_URL}//kyma-project.io/module/template-operator:${RELEASE_VERSION}" -o yaml

cat template.yaml
echo "ModuleTemplate created successfully"

if [ "${DEPLOY_MODULETEMPLATE}" == "true" ]; then
kubectl apply -f template.yaml
rm -f template.yaml
fi

rm -f module-config-for-e2e.yaml
rm -f template-operator.yaml
rm -f default-sample-cr.yaml
echo "Temporary files removed successfully"
