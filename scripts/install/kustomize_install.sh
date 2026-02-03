#!/usr/bin/env bash
# This script installs the specified version of the kustomize tool in the CURRENT_DIRECTORY.

set -o nounset
set -o errexit
set -E
set -o pipefail

KUSTOMIZE_VERSION="$1"

if [ -z "${KUSTOMIZE_VERSION}" ]; then
    echo "Error: kustomize version not provided as an argument" >&2
    echo "Usage: $0 <kustomize-version>" >&2
    exit 1
fi

# Skip installation if already installed.
if [ -d "kustomize_${KUSTOMIZE_VERSION}" ]; then
    # if the symlink is already pointing to the correct version, skip it.
    if [ -L "kustomize" ] && [ "$(readlink kustomize)" == "$(pwd)/kustomize_${KUSTOMIZE_VERSION}/kustomize" ]; then
	echo "kustomize version ${KUSTOMIZE_VERSION} is already installed and linked."
	exit 0
    fi
fi

mkdir -p "kustomize_${KUSTOMIZE_VERSION}"
pushd "kustomize_${KUSTOMIZE_VERSION}" > /dev/null
GOBIN=$(pwd) GOFIPS140=v1.0.0 go install "sigs.k8s.io/kustomize/kustomize/v5@v${KUSTOMIZE_VERSION}"
popd > /dev/null
ln -sf "$(pwd)/kustomize_${KUSTOMIZE_VERSION}/kustomize" "$(pwd)/kustomize"
echo "kustomize version ${KUSTOMIZE_VERSION} is installed."
