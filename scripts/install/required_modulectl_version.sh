#!/usr/bin/env bash
# This script outputs the required version of the modulectl tool.

set -o nounset
set -o errexit
set -E
set -o pipefail


# Find the script's directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Read the modulectl version from the versions.yaml file
MODULECTL_VERSION=$(yq -e '.modulectl' "${LIFECYCLE_MANAGER_DIR}/versions.yaml")

if [ -z "${MODULECTL_VERSION}" ]; then
    echo "Error: modulectl version not found in versions.yaml" >&2
    exit 1
fi

echo "${MODULECTL_VERSION}"
