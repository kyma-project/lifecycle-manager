#!/usr/bin/env bash
#
# chart-sync.sh - regenerate the in-repo Helm chart's generated files from the
# repository's kustomize configuration and RBAC source, killing the manual sync
# that previously coupled chart updates to management-plane-charts releases.
#
# Two independent transforms:
#
#   crds : `kustomize build config/control-plane`, filtered to the five shipped
#          CRDs, written to chart/lifecycle-manager/files/imports/crds.yaml.
#          The control-plane build (not raw config/crd/bases) is required
#          because the chart's CRDs carry metadata layered by kustomize:
#            - cert-manager.io/inject-ca-from annotation (config/certmanager)
#            - app.kubernetes.io/* commonLabels     (config/control-plane)
#          A raw concat of config/crd/bases would strip these and break the
#          production render parity gate.
#
#   rbac : the `.rules` array extracted from each hand-maintained
#          config/rbac/*.yaml source into the chart's bare-.rules fragment
#          files. The controller does not use +kubebuilder:rbac markers, so
#          these role files are source of truth, not controller-gen output.
#          The certmanager and gardener-cm fragments have no config/rbac
#          counterpart and are left untouched (static).
#
# Usage:
#   chart-sync.sh crds   [kustomize-bin]
#   chart-sync.sh rbac
#   chart-sync.sh all    [kustomize-bin]
#
# kustomize-bin defaults to `kustomize` on PATH.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_FILES="${REPO_ROOT}/chart/lifecycle-manager/files"
CRDS_OUT="${CHART_FILES}/imports/crds.yaml"
RBAC_DIR="${CHART_FILES}/rbac-rules"

# The five shipped CRDs, in the order the control-plane build emits them
# (alphabetical). This mirrors the allowlist in config/crd/kustomization.yaml
# and excludes kcpmodules (internal) and testapis (test-only).
SHIPPED_CRDS=(
  kymas.operator.kyma-project.io
  manifests.operator.kyma-project.io
  modulereleasemetas.operator.kyma-project.io
  moduletemplates.operator.kyma-project.io
  watchers.operator.kyma-project.io
)

# RBAC source file -> chart fragment file name.
RBAC_MAP=(
  "config/rbac/manager_role.yaml:klm-manager-rules.yaml"
  "config/rbac/crd_cluster_role.yaml:klm-manager-rules-crd.yaml"
  "config/rbac/leader_election_role.yaml:klm-leader-election-rules.yaml"
)

sync_crds() {
  local kustomize_bin="${1:-kustomize}"
  local built
  built="$(mktemp)"
  trap 'rm -f "$built"' RETURN

  echo "chart-sync: building CRDs from config/control-plane" >&2
  "$kustomize_bin" build "${REPO_ROOT}/config/control-plane" > "$built"

  # The generated files are not committed, so their parent directory may be
  # absent on a fresh checkout (git does not track empty directories).
  mkdir -p "$(dirname "$CRDS_OUT")"
  : > "$CRDS_OUT"
  for crd in "${SHIPPED_CRDS[@]}"; do
    {
      echo "---"
      # Extract exactly this CRD as a single document, preserving field order.
      yq ea "select(.kind == \"CustomResourceDefinition\" and .metadata.name == \"${crd}\")" "$built"
    } >> "$CRDS_OUT"
  done
  echo "chart-sync: wrote ${#SHIPPED_CRDS[@]} CRDs -> ${CRDS_OUT#"$REPO_ROOT"/}" >&2
}

sync_rbac() {
  # The generated fragments are not committed; ensure the directory exists even
  # if a fresh checkout has no committed files under it.
  mkdir -p "$RBAC_DIR"
  for pair in "${RBAC_MAP[@]}"; do
    local src="${REPO_ROOT}/${pair%%:*}"
    local frag="${RBAC_DIR}/${pair##*:}"
    [[ -f "$src" ]] || { echo "error: RBAC source not found: $src" >&2; exit 1; }
    # Emit the bare .rules array; the kind/metadata wrapper is supplied by the
    # chart's serviceaccount.yaml via .Files.Get.
    yq ea '.rules' "$src" > "$frag"
    echo "chart-sync: ${pair%%:*} .rules -> ${frag#"$REPO_ROOT"/}" >&2
  done
}

case "${1:-}" in
  crds) sync_crds "${2:-kustomize}" ;;
  rbac) sync_rbac ;;
  all)  sync_crds "${2:-kustomize}"; sync_rbac ;;
  *) echo "usage: $(basename "$0") crds|rbac|all [kustomize-bin]" >&2; exit 2 ;;
esac
