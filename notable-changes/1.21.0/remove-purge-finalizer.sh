#!/usr/bin/env bash
# Removes the orphaned purge finalizer (operator.kyma-project.io/purge-finalizer) from all Kyma
# CRs in the cluster. The purge controller was dropped in KLM <VERSION> and no longer removes this
# finalizer, so Kymas carrying it would be stuck in deletion indefinitely.
#
# IMPORTANT: Run this script immediately after updating KLM to v1.21.0 or greater. If you run it
# before updating KLM the finalizer will be re-added by the old controller. If you update KLM but
# delay running this script, any Kyma deletion attempted in the interim will block forever.
#
# Usage:
#   ./remove-purge-finalizer.sh [--execute]
#
# Flags:
#   --execute              Actually remove finalizers. Omitting this flag runs in dry-run mode, which is the default.
#
# Requirements: kubectl, yq

set -o nounset
set -o errexit
set -o pipefail

export FINALIZER="operator.kyma-project.io/purge-finalizer"
readonly KLM_DEPLOYMENT="klm-controller-manager"
readonly KLM_NAMESPACE="kcp-system"
readonly MIN_VERSION="v1.21.0"

EXECUTE=false

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
usage() {
  sed -n '/^# Usage:/,/^[^#]/p' "$0" | grep '^#' | sed 's/^# \?//'
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --execute)
    EXECUTE=true
    shift
    ;;
  --help | -h)
    usage
    ;;
  *)
    echo "ERROR: Unknown argument: $1" >&2
    usage
    ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Compares two semver strings. Returns 0 if $1 >= $2, 1 otherwise.
# Only handles MAJOR.MINOR.PATCH; pre-release suffixes are ignored.
semver_gte() {
  local actual="$1"
  local required="$2"

  local -a a r
  IFS='.' read -ra a <<<"${actual#v}"
  IFS='.' read -ra r <<<"${required#v}"

  for i in 0 1 2; do
    local av="${a[$i]:-0}"
    local rv="${r[$i]:-0}"
    if ((av > rv)); then return 0; fi
    if ((av < rv)); then return 1; fi
  done
  return 0
}

# Returns true if the string looks like a semver (optional v prefix, digits.digits.digits...).
is_semver() {
  [[ "$1" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+ ]]
}

# ---------------------------------------------------------------------------
# Version sanity check
# ---------------------------------------------------------------------------
echo "Checking KLM version in ${KLM_NAMESPACE}/${KLM_DEPLOYMENT}..."

KLM_TAG=$(kubectl get deployment "${KLM_DEPLOYMENT}" \
  -n "${KLM_NAMESPACE}" \
  -o yaml 2>/dev/null | yq '.metadata.labels["app.kubernetes.io/version"]' - || true)

VERSION_OK=false
if [[ -z "$KLM_TAG" || "$KLM_TAG" == "null" ]]; then
  echo "WARNING: Could not retrieve the 'app.kubernetes.io/version' label from deployment '${KLM_DEPLOYMENT}' in namespace '${KLM_NAMESPACE}'." >&2
elif ! is_semver "$KLM_TAG"; then
  echo "WARNING: KLM version label '${KLM_TAG}' is not a recognisable semver." >&2
elif ! semver_gte "$KLM_TAG" "$MIN_VERSION"; then
  echo "WARNING: Deployed KLM version '${KLM_TAG}' is older than the required '${MIN_VERSION}'." >&2
else
  echo "KLM version check passed: ${KLM_TAG} >= ${MIN_VERSION}"
  VERSION_OK=true
fi

if [[ "$VERSION_OK" == "false" ]]; then
  echo ""
  read -r -p "Version check failed. Have you manually verified that the deployed KLM version is >= ${MIN_VERSION}? (YES/NO): " CONFIRMED
  if [[ "$CONFIRMED" != "YES" ]]; then
    echo "Aborting. Update KLM to ${MIN_VERSION} or greater before running this script."
    exit 0
  fi
fi

# ---------------------------------------------------------------------------
# Find Kyma CRs carrying the purge finalizer
# ---------------------------------------------------------------------------
echo "Searching for Kyma CRs with finalizer '${FINALIZER}'..."

# Build a list of "namespace name" pairs for Kymas that carry the finalizer.
CANDIDATES=$(kubectl get kymas.operator.kyma-project.io \
  --all-namespaces \
  -o yaml 2>/dev/null | yq '[.items[] | select(.metadata.finalizers[] | contains(env(FINALIZER))) | [.metadata.namespace, .metadata.name] | join(" ")] | .[]' -)

if [[ -z "$CANDIDATES" ]]; then
  echo "No Kyma CRs found with the purge finalizer. Nothing to do."
  exit 0
fi

CANDIDATE_COUNT=$(echo "$CANDIDATES" | wc -l | tr -d ' ')
echo "Found ${CANDIDATE_COUNT} Kyma CR(s) with the purge finalizer."

if [[ "$EXECUTE" == "false" ]]; then
  echo ""
  echo "DRY-RUN mode (pass --execute to apply changes):"
  echo ""
  while IFS=' ' read -r namespace name; do
    echo "  ${namespace}/${name}"
  done <<<"$CANDIDATES"
  echo ""
  echo "Dry-run complete. ${CANDIDATE_COUNT} Kyma CR(s) would be patched."
  echo "Re-run with --execute to apply."
  exit 0
fi

# ---------------------------------------------------------------------------
# Remove finalizer
# ---------------------------------------------------------------------------
PATCHED=0
FAILED=0

while IFS=' ' read -r namespace name; do
  echo "  Removing finalizer from Kyma '${namespace}/${name}'..."
  FILTERED_FINALIZERS=$(kubectl get kyma.operator.kyma-project.io "${name}" -n "${namespace}" -o yaml |
    yq -o=json '[.metadata.finalizers[] | select(. != env(FINALIZER))]' - | tr -d '\n')
  if kubectl patch kyma.operator.kyma-project.io "${name}" \
    -n "${namespace}" \
    --type=merge \
    -p "{\"metadata\":{\"finalizers\":${FILTERED_FINALIZERS}}}" \
    >/dev/null; then
    echo "  OK: ${namespace}/${name}"
    ((PATCHED++)) || true
  else
    echo "  FAILED: ${namespace}/${name}" >&2
    ((FAILED++)) || true
  fi
done <<<"$CANDIDATES"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "Done. Patched: ${PATCHED}, Failed: ${FAILED}."
if ((FAILED > 0)); then
  exit 1
fi
