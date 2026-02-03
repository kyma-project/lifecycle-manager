#!/usr/bin/env bash
# This script installs the specified version of the istioctl tool in the CURRENT_DIRECTORY.

set -o nounset
set -o errexit
set -E
set -o pipefail

REQUIRED_VERSION="$1"

if [ -z "${REQUIRED_VERSION}" ]; then
    echo "Error: istioctl version not provided as an argument" >&2
    echo "Usage: $0 <istioctl-version>" >&2
    exit 1
fi

# Check if istioctl binary already exists and matches the requested version
if [ -x ./istioctl ]; then
    # Extract version from the output: "client version: 1.28.2"
    CURRENT_VERSION="$(./istioctl version --remote=false 2>&1 | grep 'client version' | awk '{print $3}')"
    if echo "${CURRENT_VERSION}" | grep -q "${REQUIRED_VERSION}"; then
	echo "istioctl version ${REQUIRED_VERSION} is already installed."
	exit 0
    fi
fi

# Check the operating system and architecture to determine the correct istioctl binary to download
ARCH_TYPE="$(uname -m)"

# Download and install the specified istioctl version for the detected OS
curl -L https://istio.io/downloadIstio | ISTIO_VERSION="${REQUIRED_VERSION}" TARGET_ARCH="${ARCH_TYPE}" sh -
chmod +x "istio-${REQUIRED_VERSION}/bin/istioctl"
rm -f istioctl
ln -s "$(pwd)/istio-${REQUIRED_VERSION}/bin/istioctl" istioctl

echo "istioctl version ${REQUIRED_VERSION} is installed."
