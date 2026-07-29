#!/usr/bin/env bash
#
# render.sh - render a lifecycle-manager Helm chart with pinned production-like
# inputs, for one certificate backend.
#
# The production render branches on whether the cert-manager.io/v1 API is
# available (.Capabilities.APIVersions.Has "cert-manager.io/v1"):
#   --cm   -> cert-manager backend  (adds the cert-manager.io/v1 capability)
#   --gcm  -> gardener backend      (omits it; chart falls back to cert.gardener.cloud)
#
# The chart path is an argument so this renders today's mp-charts chart and,
# from Phase 1 on, the in-repo candidate chart with byte-identical inputs.
#
# Usage:
#   render.sh --cm  <chart-path> [-- extra helm args...]
#   render.sh --gcm <chart-path> [-- extra helm args...]
#
# Output: rendered multi-document YAML on stdout. Diagnostics go to stderr.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALUES="${SCRIPT_DIR}/values-prod.yaml"

# Pinned release identity — labels and names depend on these, so the golden and
# candidate renders must share them (plan §6.2).
RELEASE_NAME="kcp-lifecycle-manager"
RELEASE_NS="kcp-system"

usage() {
  echo "usage: $(basename "$0") --cm|--gcm <chart-path> [-- extra helm args...]" >&2
  exit 2
}

[[ $# -ge 2 ]] || usage

BACKEND=""
case "$1" in
  --cm)  BACKEND="cm"  ;;
  --gcm) BACKEND="gcm" ;;
  *) usage ;;
esac
shift

CHART_PATH="$1"
shift

# Drop an optional "--" separator before pass-through helm args.
if [[ "${1:-}" == "--" ]]; then shift; fi

[[ -e "$CHART_PATH" ]] || { echo "error: chart path not found: $CHART_PATH" >&2; exit 1; }
[[ -f "$VALUES" ]]     || { echo "error: values file not found: $VALUES" >&2; exit 1; }

# API versions always present in the KCP landscape the chart targets.
API_VERSIONS=(
  --api-versions security.istio.io/v1beta1
  --api-versions networking.istio.io/v1beta1
  --api-versions operator.kyma-project.io/v1beta2/Watcher
)

# The cert backend toggle: presence of cert-manager.io/v1 selects the CM path.
if [[ "$BACKEND" == "cm" ]]; then
  API_VERSIONS+=(--api-versions cert-manager.io/v1)
fi

exec helm template "$RELEASE_NAME" "$CHART_PATH" \
  -n "$RELEASE_NS" \
  -f "$VALUES" \
  "${API_VERSIONS[@]}" \
  "$@"
