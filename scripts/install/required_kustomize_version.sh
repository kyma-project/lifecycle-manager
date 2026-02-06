#!/usr/bin/env bash
# This script outputs the required version of the kustomize tool.

set -o nounset
set -o errexit
set -E
set -o pipefail


# Find the script's directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Read the kustomize version from the versions.yaml file
KUSTOMIZE_VERSION=$(yq -e '.kustomize' "${LIFECYCLE_MANAGER_DIR}/versions.yaml")

if [ -z "${KUSTOMIZE_VERSION}" ]; then
    echo "Error: kustomize version not found in versions.yaml" >&2
    exit 1
fi

echo "${KUSTOMIZE_VERSION}"
