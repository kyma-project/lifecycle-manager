#!/usr/bin/env bash
#
# deploy_klm_helm.sh - deploy KLM into the local KCP k3d cluster using the
# in-repo Helm chart (chart/lifecycle-manager) with the local k3d values.
#
# This is the Helm equivalent of deploy_klm_from_sources.sh: it builds and
# pushes the image the same way, then deploys via `helm upgrade --install`
# instead of `kustomize build | kubectl apply`. It is selected by USE_HELM=true
# in the e2e deploy-klm target.
#
# The certificate backend is chosen automatically from the cluster's
# capabilities (whichever cert stack create_test_clusters.sh installed), so
# there is no --use-gcm switch here; the chart branches on
# .Capabilities.APIVersions.Has "cert-manager.io/v1".
set -o nounset
set -o errexit
set -E
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

CHART_DIR="chart/lifecycle-manager"
VALUES_LOCAL="${CHART_DIR}/values-local.yaml"
RELEASE_NAME="kcp-lifecycle-manager"
NAMESPACE="kcp-system"

export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml
export LOCAL_IMG="localhost:5111/lifecycle-manager"
export CLUSTER_IMG="k3d-kcp-registry.localhost:5000/lifecycle-manager"
export TAG=$(date +%Y%m%d%H%M%S)

make docker-build IMG=${LOCAL_IMG}:${TAG}
make docker-push IMG=${LOCAL_IMG}:${TAG}

# Regenerate the chart's CRDs and RBAC from the current sources so local testing
# exercises the same generated content the parity gate checks.
make chart-sync

maxRetry=5
for retry in $(seq 1 $maxRetry)
do
  if helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
      --namespace "${NAMESPACE}" --create-namespace \
      -f "${VALUES_LOCAL}" \
      --set image.repository="${CLUSTER_IMG}" \
      --set image.tag="${TAG}" \
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
