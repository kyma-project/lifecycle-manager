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
OS_TYPE="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH_TYPE="$(uname -m)"
if [ "${OS_TYPE}" = "darwin" ]; then
    if [ "${ARCH_TYPE}" = "arm64" ]; then
	OS_ARCH="osx-arm64"
    else
	OS_ARCH="osx"
    fi
else
    if [ "${ARCH_TYPE}" = "x86_64" ]; then
	OS_ARCH="linux-amd64"
    else
	OS_ARCH="linux-arm64"
    fi
fi

# Read the pinned SHA-256 checksum for the detected OS/architecture from versions.yaml
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
EXPECTED_CHECKSUM="$(yq -e ".[\"istio-checksum-${OS_ARCH}\"]" "${LIFECYCLE_MANAGER_DIR}/versions.yaml")"

# Download the specified istio release tarball and verify it against the pinned checksum
ARCHIVE="istio-${REQUIRED_VERSION}-${OS_ARCH}.tar.gz"
echo "Downloading istio version ${REQUIRED_VERSION} (${OS_ARCH})..."
curl -fsSL -o "${ARCHIVE}" "https://github.com/istio/istio/releases/download/${REQUIRED_VERSION}/${ARCHIVE}"

if command -v sha256sum > /dev/null 2>&1; then
    ACTUAL_CHECKSUM="$(sha256sum "${ARCHIVE}" | awk '{print $1}')"
else
    ACTUAL_CHECKSUM="$(shasum -a 256 "${ARCHIVE}" | awk '{print $1}')"
fi

if [ "${ACTUAL_CHECKSUM}" != "${EXPECTED_CHECKSUM}" ]; then
    echo "Error: checksum mismatch for ${ARCHIVE}" >&2
    echo "  expected: ${EXPECTED_CHECKSUM}" >&2
    echo "  actual:   ${ACTUAL_CHECKSUM}" >&2
    rm -f "${ARCHIVE}"
    exit 1
fi

tar -xzf "${ARCHIVE}"
rm -f "${ARCHIVE}"
chmod +x "istio-${REQUIRED_VERSION}/bin/istioctl"
rm -f istioctl
ln -s "$(pwd)/istio-${REQUIRED_VERSION}/bin/istioctl" istioctl

echo "istioctl version ${REQUIRED_VERSION} is installed."
