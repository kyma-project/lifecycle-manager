#!/bin/bash

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 [name of test target]"
  exit 1
fi

# Changing to root directory of the repository
cd "$(git rev-parse --show-toplevel)"

# Exporting necessary environment variables
export KCP_KUBECONFIG=${HOME}/.k3d/kcp-local.yaml
export SKR_KUBECONFIG=${HOME}/.k3d/skr-local.yaml

# Execute E2E Tests
echo "[$(basename $0)] Running E2E Tests"
# Error handling for invalid test target shall be handled by the Makefile
make -C tests/e2e $1
