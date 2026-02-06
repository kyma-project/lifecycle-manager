#!/usr/bin/env bash
# This script installs the specified version of the modulectl tool in the CURRENT_DIRECTORY.
# TODO: Maybe it is better if modulectl can be installed via 'go install'?

set -o nounset
set -o errexit
set -E
set -o pipefail

REQUIRED_VERSION="$1"

if [ -z "${REQUIRED_VERSION}" ]; then
    echo "Error: modulectl version not provided as an argument" >&2
    echo "Usage: $0 <modulectl-version>" >&2
    exit 1
fi

# Check if modulectl binary already exists and matches the requested version
if [ -x ./modulectl ]; then
    CURRENT_VERSION="$(./modulectl version 2>&1)"
    if echo "${CURRENT_VERSION}" | grep -q "${REQUIRED_VERSION}"; then
	echo "modulectl version ${REQUIRED_VERSION} is already installed."
	exit 0
    fi
fi

# Check the operating system and architecture to determine the correct modulectl binary to download
OS_TYPE="$(uname | tr '[:upper:]' '[:lower:]')"
if [ "${OS_TYPE}" = "darwin" ]; then
    ARCH_TYPE="$(uname -m)"
    if [ "${ARCH_TYPE}" = "arm64" ]; then
	FILE_NAME="modulectl-darwin-arm"
    else
	FILE_NAME="modulectl-darwin"
    fi
else
    ARCH_TYPE="$(uname -m)"
    if [ "${ARCH_TYPE}" = "x86_64" ]; then
	FILE_NAME="modulectl-linux"
    else
	FILE_NAME="modulectl-linux-arm"
    fi
fi

# Download and install the specified modulectl version for the detected OS
echo "Downloading modulectl version ${REQUIRED_VERSION}..."
wget "https://github.com/kyma-project/modulectl/releases/download/${REQUIRED_VERSION}/${FILE_NAME}" -O modulectl > /dev/null 2>&1
chmod +x ./modulectl
echo "modulectl version ${REQUIRED_VERSION} is installed."
