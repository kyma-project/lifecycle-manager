#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

kubectl patch svc klm-controller-manager-metrics -p '{"spec": {"type": "LoadBalancer"}}' -n kcp-system
