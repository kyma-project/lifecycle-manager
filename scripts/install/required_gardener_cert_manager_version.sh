#!/usr/bin/env bash
# This script outputs the required version of Gardener CertManager when running e2e tests.

set -o nounset
set -o errexit
set -E
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_MANAGER_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

GARDENER_CERT_MANAGER_VERSION=$(yq -e '.gardenerCertManager' "${LIFECYCLE_MANAGER_DIR}/versions.yaml")

if [ -z "${GARDENER_CERT_MANAGER_VERSION}" ]; then
    echo "Error: gardenerCertManager version not found in versions.yaml" >&2
    exit 1
fi

echo "${GARDENER_CERT_MANAGER_VERSION}"
