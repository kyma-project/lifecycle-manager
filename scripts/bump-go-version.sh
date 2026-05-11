#!/usr/bin/env bash
set -euo pipefail

# Usage: ./scripts/bump-go-version.sh <new-go-version>
# Example: ./scripts/bump-go-version.sh 1.26.3

NEW_VERSION="${1:?Usage: $0 <new-go-version>}"

# When piped to bash (e.g. curl ... | bash), BASH_SOURCE[0] is not a file path,
# so fall back to the current working directory as the repo root.
if [[ -f "${BASH_SOURCE[0]:-}" ]]; then
  REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
else
  REPO_ROOT="$(pwd)"
fi

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
while IFS= read -r modfile; do
  moddir="$(dirname "${modfile}")"
  (cd "${moddir}" && go mod edit -go="${NEW_VERSION}")
  echo "  ✓ ${modfile#${REPO_ROOT}/}"
done < <(find "${REPO_ROOT}" -name "go.mod" -not -path "*/vendor/*")

# 2. Resolve Docker image index digest (validates the image exists on the registry)
IMAGE="golang:${NEW_VERSION}-alpine"
DIGEST=$(docker buildx imagetools inspect "${IMAGE}" --format '{{println .Manifest.Digest}}')

if [[ -z "${DIGEST}" ]]; then
  echo "  ✗ Failed to resolve digest for ${IMAGE} image. Does this Go version exist?" >&2
  exit 1
fi
echo "  ✓ Verified ${IMAGE} image exists (${DIGEST})"

# 3. Update versions.yaml (optional: only if the file exists and contains a "go" entry)
VERSIONS_YAML="${REPO_ROOT}/versions.yaml"
if [[ -f "${VERSIONS_YAML}" ]] && yq e '.go' "${VERSIONS_YAML}" | grep -qv '^null$'; then
  yq e -i ".go = \"${NEW_VERSION}\"" "${VERSIONS_YAML}"
  echo "  ✓ versions.yaml"
fi

# 4. Update all Dockerfiles with new version and resolved digest
while IFS= read -r dockerfile; do
  sedi "s|golang:[0-9]\+\.[0-9]\+\.[0-9]\+-alpine@sha256:[a-f0-9]\+|golang:${NEW_VERSION}-alpine@${DIGEST}|" "${dockerfile}"
  echo "  ✓ ${dockerfile#${REPO_ROOT}/} (golang:${NEW_VERSION}-alpine@${DIGEST})"
done < <(find "${REPO_ROOT}" -name "Dockerfile" -not -path "*/vendor/*")

echo ""
echo "Done! Go version bumped to ${NEW_VERSION}."
