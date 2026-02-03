#!/usr/bin/env bash
# This script installs the specified version of the ginkgo tool in the CURRENT_DIRECTORY.

set -o nounset
set -o errexit
set -E
set -o pipefail

GINKGO_VERSION="$1"

if [ -z "${GINKGO_VERSION}" ]; then
    echo "Error: ginkgo version not provided as an argument" >&2
    echo "Usage: $0 <ginkgo-version>" >&2
    exit 1
fi

# Skip installation if already installed.
if [ -d "ginkgo_${GINKGO_VERSION}" ]; then
    # if the symlink is already pointing to the correct version, skip it.
    if [ -L "ginkgo" ] && [ "$(readlink ginkgo)" == "$(pwd)/ginkgo_${GINKGO_VERSION}/ginkgo" ]; then
	echo "ginkgo version ${GINKGO_VERSION} is already installed and linked."
	exit 0
    fi
fi

mkdir -p "ginkgo_${GINKGO_VERSION}"
pushd "ginkgo_${GINKGO_VERSION}" > /dev/null
GOBIN=$(pwd) GOFIPS140=v1.0.0 go install "github.com/onsi/ginkgo/v2/ginkgo@v${GINKGO_VERSION}"
popd > /dev/null
ln -sf "$(pwd)/ginkgo_${GINKGO_VERSION}/ginkgo" "$(pwd)/ginkgo"
echo "ginkgo version ${GINKGO_VERSION} is installed."
