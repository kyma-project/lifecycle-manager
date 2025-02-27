#!/bin/bash

k3d kubeconfig merge -a -d
export KCP_KUBECONFIG=$(k3d kubeconfig write skr)
export SKR_KUBECONFIG=$(k3d kubeconfig write skr)
kubectl config use-context k3d-kcp
