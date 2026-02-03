#!/usr/bin/env bash
# This script outputs the required version of the istioctl tool.

set -o nounset
set -o errexit
set -E
set -o pipefail


# Find the script's directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Read the istioctl version from the versions.yaml file
ISTIO_VERSION=$(yq -e '.istio' "${LIFECYCLE_MANAGER_DIR}/versions.yaml")

if [ -z "${ISTIO_VERSION}" ]; then
    echo "Error: istio version not found in versions.yaml" >&2
    exit 1
fi

echo "${ISTIO_VERSION}"
