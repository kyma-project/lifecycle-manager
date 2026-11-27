#!/usr/bin/env bash
set -euo pipefail

# Updates the istio-checksum-* keys in versions.yaml for the currently pinned istio version
# by fetching istio's published .sha256 for each os-arch release tarball.

if [[ -f "${BASH_SOURCE[0]:-}" ]]; then
  REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
else
  REPO_ROOT="$(pwd)"
fi

VERSIONS_FILE="${REPO_ROOT}/versions.yaml"
ISTIO_VERSION="$(yq -e '.istio' "${VERSIONS_FILE}")"
OS_ARCHS=(osx osx-arm64 linux-amd64 linux-arm64)

echo "Updating istio checksums for istio ${ISTIO_VERSION}..."

for OS_ARCH in "${OS_ARCHS[@]}"; do
  URL="https://github.com/istio/istio/releases/download/${ISTIO_VERSION}/istio-${ISTIO_VERSION}-${OS_ARCH}.tar.gz.sha256"
  CHECKSUM="$(curl -fsSL "${URL}" | awk '{print $1}')"

  CHECKSUM="${CHECKSUM}" OS_ARCH="${OS_ARCH}" \
    yq -i '.["istio-checksum-" + strenv(OS_ARCH)] = strenv(CHECKSUM)' "${VERSIONS_FILE}"
  echo "  ✓ ${OS_ARCH}: ${CHECKSUM}"
done

echo "Done. Review the changes in versions.yaml before committing."
