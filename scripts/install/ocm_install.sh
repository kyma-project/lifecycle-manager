#!/usr/bin/env bash
# This script installs the specified version of the ocm tool in the CURRENT_DIRECTORY.

set -o nounset
set -o errexit
set -E
set -o pipefail

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <version>" >&2
    echo "Example: $0 0.32.0" >&2
    exit 1
fi

VERSION=$1
if [ -z "${VERSION}" ]; then
    echo "Version input empty; aborting." >&2
    exit 1
fi

if [ "${VERSION}" = "latest" ] || [ "${VERSION}" = "main" ]; then
    echo "Dynamic versions ('latest'/'main') are not allowed; provide an explicit version like 0.32.0." >&2
    exit 1
fi

if ! echo "${VERSION}" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "Version '${VERSION}' does not match required pattern MAJOR.MINOR.PATCH." >&2
    exit 1
fi

# test if ocm binary exists in the current directory
if [ -x "./ocm" ]; then
    CURRENT_VERSION="$(./ocm version | yq -p json -oy '.GitVersion')"
    if [ "${CURRENT_VERSION}" = "${VERSION}" ]; then
	echo "OCM CLI version ${VERSION} is already installed."
	exit 0
    fi
fi

echo "Installing required version: ${VERSION}"
VERSION_NO_V="${VERSION#v}"
# Create temporary installation directory and set DOWNLOAD_PATH to it
DOWNLOAD_PATH="$(mktemp -d)/ocm-cli-bin"
mkdir -p "${DOWNLOAD_PATH}"

# Check the operating system and architecture to determine the correct modulectl binary to download
FILE_SUFFIX="linux-amd64.tar.gz" # example:  ocm-0.35.0-linux-amd64.tar.gz 
OS_TYPE="$(uname | tr '[:upper:]' '[:lower:]')"
if [ "${OS_TYPE}" = "darwin" ]; then
    ARCH_TYPE="$(uname -m)"
    if [ "${ARCH_TYPE}" = "arm64" ]; then
	FILE_SUFFIX="darwin-arm64.tar.gz" # example: ocm-0.35.0-darwin-arm64.tar.gz
    else
	FILE_SUFFIX="darwin-amd64.tar.gz" # example: ocm-0.35.0-darwin-amd64.tar.gz
    fi
fi

ARCHIVE="ocm-${VERSION_NO_V}-${FILE_SUFFIX}"
BINARY_URL="https://github.com/open-component-model/ocm/releases/download/v${VERSION}/${ARCHIVE}"
SHA_URL="${BINARY_URL}.sha256"

echo "Downloading ${ARCHIVE}"
curl -sSfL "${BINARY_URL}" -o "${DOWNLOAD_PATH}/${ARCHIVE}"

echo "Verifying checksum"
curl -sSfL "${SHA_URL}" -o "${DOWNLOAD_PATH}/${ARCHIVE}.sha256"
EXPECTED_SHA="$(cat "${DOWNLOAD_PATH}/${ARCHIVE}.sha256" | tr -d '\n\r')"
echo "${EXPECTED_SHA}  ${DOWNLOAD_PATH}/${ARCHIVE}" | sha256sum -c -

tar -xzf "${DOWNLOAD_PATH}/${ARCHIVE}" -C "${DOWNLOAD_PATH}"
if [ ! -x "${DOWNLOAD_PATH}/ocm" ]; then
  echo "ocm binary not found after extraction" >&2
  exit 1
fi

mv "${DOWNLOAD_PATH}/ocm" .
chmod +x ./ocm
INSTALLED_VERSION="$(./ocm version | yq -p json -oy '.GitVersion')"
echo "ocm version ${INSTALLED_VERSION} is installed."
