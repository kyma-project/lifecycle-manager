#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

# Remove the k3d cluster and the skr cluster
k3d cluster rm kcp skr

docker rm -f private-oci-reg.localhost 2>/dev/null || true

echo "[$(basename $0)] Cleanup completed"
