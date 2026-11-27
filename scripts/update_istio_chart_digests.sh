#!/usr/bin/env bash
set -euo pipefail

# Updates the istio-chart-* digests in versions.yaml for the currently pinned istio
# version (the `istio:` key). Resolves each chart's SHA-256 digest from the OCI registry
# via `helm show chart` and writes it back.
#
# These digests are the supply-chain trust anchor: review the regenerated values in the
# PR. From then on, helm verifies every chart pull against them.

# When piped to bash (e.g. curl ... | bash), BASH_SOURCE[0] is not a file path,
# so fall back to the current working directory as the repo root.
if [[ -f "${BASH_SOURCE[0]:-}" ]]; then
  REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
else
  REPO_ROOT="$(pwd)"
fi

VERSIONS_FILE="${REPO_ROOT}/versions.yaml"
ISTIO_REGISTRY="oci://gcr.io/istio-release/charts"

ISTIO_VERSION="$(yq -e '.istio' "${VERSIONS_FILE}")"
CHARTS=(base istiod gateway)

echo "Updating istio chart digests for istio ${ISTIO_VERSION}..."

for CHART in "${CHARTS[@]}"; do
  # `helm show chart` resolves the tag against the registry and prints the digest to
  # stderr; redirect stdout away so only the digest line is parsed.
  DIGEST="$(helm show chart "${ISTIO_REGISTRY}/${CHART}" --version "${ISTIO_VERSION}" 2>&1 >/dev/null \
    | awk '/Digest:/ {print $2}')"

  if [[ -z "${DIGEST}" ]]; then
    echo "Error: could not resolve digest for ${CHART} chart ${ISTIO_VERSION}" >&2
    exit 1
  fi

  DIGEST="${DIGEST}" CHART="${CHART}" \
    yq -i '.["istio-chart-" + strenv(CHART)] = strenv(DIGEST)' "${VERSIONS_FILE}"
  echo "  ✓ istio-chart-${CHART}: ${DIGEST}"
done

echo "Done. Review the changes in versions.yaml before committing."
