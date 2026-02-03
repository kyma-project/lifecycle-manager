#!/usr/bin/env bash
# This script outputs the required version of the ocm tool.

set -o nounset
set -o errexit
set -E
set -o pipefail


# Find the script's directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Read the ocm-cli version from the versions.yaml file
OCM_VERSION=$(yq -e '.ocm-cli' "${LIFECYCLE_MANAGER_DIR}/versions.yaml")

if [ -z "${OCM_VERSION}" ]; then
    echo "Error: ocm-cli version not found in versions.yaml" >&2
    exit 1
fi

echo "${OCM_VERSION}"
