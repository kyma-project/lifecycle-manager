#!/usr/bin/env bash
set -o pipefail
set -o errexit

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

MODULE_NAME=""
RELEASE_VERSION=""
DEPLOYMENT_NAME=""
DEPLOYABLE_VERSION=""
MANDATORY=false
INCLUDE_DEFAULT_CR=true
REQUIRES_DOWNTIME=false
DEPLOY_MODULETEMPLATE=true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --module-name)        MODULE_NAME="$2";            shift 2 ;;
    --version)            RELEASE_VERSION="$2";        shift 2 ;;
    --deployment-name)    DEPLOYMENT_NAME="$2";        shift 2 ;;
    --deployable-version) DEPLOYABLE_VERSION="$2";     shift 2 ;;
    --mandatory)          MANDATORY=true;               shift   ;;
    --no-default-cr)      INCLUDE_DEFAULT_CR=false;    shift   ;;
    --requires-downtime)  REQUIRES_DOWNTIME=true;      shift   ;;
    --skip-apply)         DEPLOY_MODULETEMPLATE=false;  shift   ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

MISSING=()
[[ -z "$MODULE_NAME" ]]        && MISSING+=("--module-name")
[[ -z "$RELEASE_VERSION" ]]    && MISSING+=("--version")
[[ -z "$DEPLOYMENT_NAME" ]]    && MISSING+=("--deployment-name")
[[ -z "$DEPLOYABLE_VERSION" ]] && MISSING+=("--deployable-version")
if [[ ${#MISSING[@]} -gt 0 ]]; then
  echo "Missing required flags: ${MISSING[*]}"; exit 1
fi

if [ ! -f deploy_moduletemplate.sh ]; then
  cp "$SCRIPT_DIR/deploy_moduletemplate.sh" .
fi
if [ ! -f ocm-config-local-registry.yaml ]; then
  cp "$SCRIPT_DIR/ocm-config-local-registry.yaml" .
fi

echo "=== Preparing module template: ${MODULE_NAME} ${RELEASE_VERSION} ==="

yq eval ".images[0].newTag = \"${RELEASE_VERSION}\"" -i config/manager/deployment/kustomization.yaml
make build-manifests
yq eval "(. | select(.kind == \"Deployment\") | .metadata.name) = \"${DEPLOYMENT_NAME}\"" -i template-operator.yaml

./deploy_moduletemplate.sh \
  "${MODULE_NAME}" \
  "${RELEASE_VERSION}" \
  "${DEPLOYABLE_VERSION}" \
  "${INCLUDE_DEFAULT_CR}" \
  "${MANDATORY}" \
  "${DEPLOY_MODULETEMPLATE}" \
  "${REQUIRES_DOWNTIME}"
