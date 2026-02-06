#!/usr/bin/env bash
# This script outputs the required version of Kubernetes for creating test clusters.

set -o nounset
set -o errexit
set -E
set -o pipefail

# Find the script's directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Read the Kubernetes version from the versions.yaml file
KUBERNETES_VERSION=$(yq -e '.k8s' "${LIFECYCLE_MANAGER_DIR}/versions.yaml")

if [ -z "${KUBERNETES_VERSION}" ]; then
    echo "Error: k8s version not found in versions.yaml" >&2
    exit 1
fi

echo "${KUBERNETES_VERSION}"
