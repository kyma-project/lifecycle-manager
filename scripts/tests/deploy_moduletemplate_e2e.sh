#!/bin/bash
set -o pipefail
set -o errexit

SCRIPT_DIR=$(dirname "$(realpath $0)")

MODULE_NAME=$1
MODULE_VERSION=$2
MODULE_DEPLOYMENT_NAME=$3
INCLUDE_DEFAULT_CR=${4:-true}
MANDATORY=${5:-false}
DEPLOY_MODULETEMPLATE=${6:-true}
REQUIRES_DOWNTIME=${7:-false}

if [ ! -f deploy_moduletemplate.sh ]; then
  cp "$SCRIPT_DIR"/deploy_moduletemplate.sh .
fi

# Workaround for the issue with the missing CA certificates
export SSL_CERT_DIR="$HOME/.local/share/ca-certificates"
export CURL_CA_BUNDLE="$SSL_CERT_DIR/server.crt"

make build-manifests
yq eval "(. | select(.kind == \"Deployment\") | .metadata.name) = \"$MODULE_DEPLOYMENT_NAME\"" -i ./template-operator.yaml
./deploy_moduletemplate.sh "$MODULE_NAME" "$MODULE_VERSION" "$INCLUDE_DEFAULT_CR" "$MANDATORY" "$DEPLOY_MODULETEMPLATE" "$REQUIRES_DOWNTIME"
