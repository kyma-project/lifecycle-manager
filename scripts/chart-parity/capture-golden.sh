#!/usr/bin/env bash
#
# capture-golden.sh - (re)generate the committed golden baseline from the
# production mp-charts chart, for both certificate backends.
#
# The golden files are the FROZEN production render. They are captured from the
# management-plane-charts chart at a pinned ref and committed to this repo so
# the parity gate runs without a cross-repo checkout (per the agreed strategy).
#
# Pinned mp-charts ref: cf59114e8  ("KLM Release 1.20.4 (#5691)")
#   This is the head the migration plan requires (§6.3): at or past release
#   1.20.4 the chart's manager Role already matches config/rbac, so the RBAC
#   drift described in plan §2.5 is not present in the baseline. Capturing
#   against an older checkout would re-introduce the resolved drift.
#
# Usage:
#   MP_CHARTS_CHART=/path/to/management-plane-charts/lifecycle-manager \
#     capture-golden.sh
#
# Defaults MP_CHARTS_CHART to the sibling-repo layout used in development.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RENDER="${SCRIPT_DIR}/render.sh"
GOLDEN_DIR="${SCRIPT_DIR}/golden"

PINNED_REF="cf59114e8"

MP_CHARTS_CHART="${MP_CHARTS_CHART:-$HOME/go/src/kyma/management-plane-charts/lifecycle-manager}"

[[ -d "$MP_CHARTS_CHART" ]] || {
  echo "error: mp-charts chart dir not found: $MP_CHARTS_CHART" >&2
  echo "       set MP_CHARTS_CHART to the lifecycle-manager chart directory." >&2
  exit 1
}

# Guard: refuse to capture unless the source repo is at the pinned ref. This
# prevents silently rebaselining the golden against a drifted checkout.
chart_repo_root="$(git -C "$MP_CHARTS_CHART" rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -n "$chart_repo_root" ]]; then
  head_sha="$(git -C "$chart_repo_root" rev-parse --short=9 HEAD)"
  if [[ "$head_sha" != "$PINNED_REF" ]]; then
    echo "error: mp-charts is at ${head_sha}, expected pinned ref ${PINNED_REF}." >&2
    echo "       checkout ${PINNED_REF} before capturing, or update PINNED_REF" >&2
    echo "       here and in README.md if intentionally rebaselining." >&2
    exit 1
  fi
  if ! git -C "$chart_repo_root" diff --quiet HEAD -- "$MP_CHARTS_CHART"; then
    echo "error: mp-charts chart has uncommitted changes; refusing to capture." >&2
    exit 1
  fi
else
  echo "warning: $MP_CHARTS_CHART is not in a git repo; cannot verify pinned ref." >&2
fi

mkdir -p "$GOLDEN_DIR"

echo "capturing golden (cert-manager backend) ..." >&2
"$RENDER" --cm  "$MP_CHARTS_CHART" > "${GOLDEN_DIR}/prod-cm.yaml"

echo "capturing golden (gardener backend) ..." >&2
"$RENDER" --gcm "$MP_CHARTS_CHART" > "${GOLDEN_DIR}/prod-gcm.yaml"

echo "golden baseline written to ${GOLDEN_DIR}:" >&2
echo "  prod-cm.yaml  (cert-manager.io/v1)" >&2
echo "  prod-gcm.yaml (cert.gardener.cloud/v1alpha1)" >&2
