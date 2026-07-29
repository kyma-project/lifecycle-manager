#!/usr/bin/env bash
#
# deploy_klm_helm.sh - deploy KLM into a KCP k3d cluster using the in-repo Helm
# chart (chart/lifecycle-manager). Selected by USE_HELM=true in the e2e
# deploy-klm target.
#
# Two modes:
#   local (default): build and push the image from source to the k3d registry,
#     regenerate the chart's CRDs/RBAC (make chart-sync), then deploy. This is
#     the Helm equivalent of deploy_klm_from_sources.sh.
#   registry (--image-registry given): deploy a pre-built registry image, the
#     way CI does. Mirrors deploy_klm_from_registry.sh; no docker build/push and
#     no chart-sync (the image is pre-built at the checked-out SHA and the
#     chart's generated files are committed and parity-gated).
#
# The certificate backend is chosen automatically from the cluster's
# capabilities (the chart branches on .Capabilities.APIVersions.Has
# "cert-manager.io/v1"). --use-gcm layers the GCM-only local overrides
# (values-local-gcm.yaml) needed by the local Gardener cert stack.
#
# Usage:
#   deploy_klm_helm.sh [--image-registry dev|prod|ghcr] [--image-tag <tag>] \
#                      [--use-gcm] [--values-overlay <path>]...
set -o nounset
set -o errexit
set -E
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

CHART_DIR="chart/lifecycle-manager"
VALUES_LOCAL="${CHART_DIR}/values-local.yaml"
VALUES_LOCAL_GCM="${CHART_DIR}/values-local-gcm.yaml"
RELEASE_NAME="kcp-lifecycle-manager"
NAMESPACE="kcp-system"

KLM_IMAGE_REGISTRY=""
KLM_IMAGE_TAG=""
USE_GCM=""
VALUES_OVERLAYS=()

while [[ "$#" -gt 0 ]]; do
  case $1 in
    --image-registry) KLM_IMAGE_REGISTRY="$2"; shift ;;
    --image-tag) KLM_IMAGE_TAG="$2"; shift ;;
    --use-gcm) USE_GCM="true" ;;
    --values-overlay) VALUES_OVERLAYS+=("$2"); shift ;;
    *)
      echo "Unknown parameter passed: $1";
      echo "Usage: $0 [--image-registry dev|prod|ghcr] [--image-tag <tag>] [--use-gcm] [--values-overlay <path>]...";
      exit 1 ;;
  esac
  shift
done

export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml

# Resolve the image reference for the two modes.
if [[ -n "${KLM_IMAGE_REGISTRY}" ]]; then
  # Registry mode (CI): consume a pre-built image, no build/push, no chart-sync.
  if [[ -z "${KLM_IMAGE_TAG}" ]]; then
    echo "Error: --image-tag is required with --image-registry"
    exit 1
  fi
  case "${KLM_IMAGE_REGISTRY}" in
    dev|prod)
      IMAGE_REPOSITORY="europe-docker.pkg.dev/kyma-project/${KLM_IMAGE_REGISTRY}/lifecycle-manager"
      ;;
    ghcr)
      IMAGE_REPOSITORY="ghcr.io/kyma-project/lifecycle-manager"
      ;;
    *)
      echo "Unknown registry '${KLM_IMAGE_REGISTRY}'. Valid options: dev, prod, ghcr"
      exit 1
      ;;
  esac
  IMAGE_TAG="${KLM_IMAGE_TAG}"
else
  # Local mode: build and push from source to the k3d registry, then sync chart.
  export LOCAL_IMG="localhost:5111/lifecycle-manager"
  export CLUSTER_IMG="k3d-kcp-registry.localhost:5000/lifecycle-manager"
  TAG=$(date +%Y%m%d%H%M%S)

  make docker-build IMG=${LOCAL_IMG}:${TAG}
  make docker-push IMG=${LOCAL_IMG}:${TAG}

  # Regenerate the chart's CRDs and RBAC from the current sources so local
  # testing exercises the same generated content the parity gate checks.
  make chart-sync

  IMAGE_REPOSITORY="${CLUSTER_IMG}"
  IMAGE_TAG="${TAG}"
fi

# Assemble the -f value files in precedence order (last wins): base local
# values, then the GCM-only overrides when requested, then any per-test overlay.
VALUES_ARGS=(-f "${VALUES_LOCAL}")
if [[ "${USE_GCM}" == "true" ]]; then
  VALUES_ARGS+=(-f "${VALUES_LOCAL_GCM}")
fi
for overlay in ${VALUES_OVERLAYS[@]+"${VALUES_OVERLAYS[@]}"}; do
  VALUES_ARGS+=(-f "${overlay}")
done

maxRetry=5
for retry in $(seq 1 $maxRetry)
do
  if helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
      --namespace "${NAMESPACE}" --create-namespace \
      "${VALUES_ARGS[@]}" \
      --set image.repository="${IMAGE_REPOSITORY}" \
      --set image.tag="${IMAGE_TAG}" \
      --wait --timeout 5m; then
    set +e
    kubectl wait pods -n "${NAMESPACE}" -l app.kubernetes.io/name=lifecycle-manager --for condition=Ready --timeout=20s
    status=$?
    set -e
    if [[ $status -ne 0 ]]; then
      echo "KLM pods are not ready yet, will retry deployment"
      continue
    fi
    echo "KLM deployed successfully via Helm"
    exit 0
  elif [[ $retry -lt $maxRetry ]]; then
    echo "Helm deploy encountered some error, will retry after 20 seconds"
    sleep 20
  else
    echo "KLM Helm deployment failed"
    exit 1
  fi
done
