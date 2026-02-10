#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail


k3d kubeconfig merge -a -d
export KCP_KUBECONFIG=$(k3d kubeconfig write kcp)
export SKR_KUBECONFIG=$(k3d kubeconfig write skr)
kubectl config use-context k3d-kcp
