#!/usr/bin/env bash
# This script outputs the required version of the ginkgo tool.

set -o nounset
set -o errexit
set -E
set -o pipefail


# Find the script's directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Read the ginkgo version from the versions.yaml file
GINKGO_VERSION=$(cat "${LIFECYCLE_MANAGER_DIR}/go.mod" | grep 'github.com/onsi/ginkgo/v2' | head -1 | awk '{print $2}' | sed 's/v//')

if [ -z "${GINKGO_VERSION}" ]; then
    echo "Error: ginkgo version not found in go.mod" >&2
    exit 1
fi

echo "${GINKGO_VERSION}"
