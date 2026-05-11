#!/usr/bin/env bash
set -euo pipefail

# Usage: ./scripts/bump-go-version.sh <new-go-version>
# Example: ./scripts/bump-go-version.sh 1.26.3

NEW_VERSION="${1:?Usage: $0 <new-go-version>}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Helper function to handle sed -i differences between GNU and BSD (macOS)
sedi() {
    if sed --version >/dev/null 2>&1; then
        sed -i "$@"
    else
        sed -i '' "$@"
    fi
}

echo "Bumping Go version to ${NEW_VERSION}..."

# 1. Update all go.mod files using 'go mod edit'
#    This validates quickly that the Go version is recognized by the toolchain.
for moddir in "${REPO_ROOT}" "${REPO_ROOT}/api" "${REPO_ROOT}/maintenancewindows"; do
  (cd "${moddir}" && go mod edit -go="${NEW_VERSION}")
  echo "  ✓ ${moddir#${REPO_ROOT}/}/go.mod"
done

# 2. Resolve Docker image index digest (validates the image exists on the registry)
IMAGE="golang:${NEW_VERSION}-alpine"
DIGEST=$(docker buildx imagetools inspect "${IMAGE}" --format '{{println .Manifest.Digest}}')

if [[ -z "${DIGEST}" ]]; then
  echo "  ✗ Failed to resolve digest for ${IMAGE} image. Does this Go version exist?" >&2
  exit 1
fi
echo "  ✓ Verified ${IMAGE} image exists (${DIGEST})"

# 3. Update versions.yaml
yq e -i ".go = \"${NEW_VERSION}\"" "${REPO_ROOT}/versions.yaml"
echo "  ✓ versions.yaml"

# 4. Update Dockerfile with new version and resolved digest
sedi "s|golang:[0-9]\+\.[0-9]\+\.[0-9]\+-alpine@sha256:[a-f0-9]\+|golang:${NEW_VERSION}-alpine@${DIGEST}|" "${REPO_ROOT}/Dockerfile"
echo "  ✓ Dockerfile (golang:${NEW_VERSION}-alpine@${DIGEST})"

echo ""
echo "Done! Go version bumped to ${NEW_VERSION}."
