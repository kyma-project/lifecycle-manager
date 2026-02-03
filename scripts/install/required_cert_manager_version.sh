#!/usr/bin/env bash
# This script outputs the required version of CertManager when running e2e tests.

set -o nounset
set -o errexit
set -E
set -o pipefail


# Find the script's directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Read the CertManager version from the versions.yaml file
CERT_MANAGER_VERSION=$(yq -e '.certManager' "${LIFECYCLE_MANAGER_DIR}/versions.yaml")

if [ -z "${CERT_MANAGER_VERSION}" ]; then
    echo "Error: CertManager version not found in versions.yaml" >&2
    exit 1
fi

echo "${CERT_MANAGER_VERSION}"
