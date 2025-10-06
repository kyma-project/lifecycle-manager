#!/bin/bash

# Change to root directory of the project
cd "$(git rev-parse --show-toplevel)"

# Exporting necessary environment variables
export KUBECONFIG=${HOME}/.k3d/kcp-local.yaml

# Install CRDs
echo "[$(basename $0)] Installing CRDs"
make install
echo "[$(basename $0)] CRDs installed successfully"
