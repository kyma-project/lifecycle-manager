#!/bin/bash

NEW_SKR_HOSTNAME="skr.cluster.local"
HOST_IP_ADDRESS=""

FOUND=0
SECONDS=0
TIMEOUT=60

until [ $FOUND -eq 1 ]; do
    if [ $SECONDS -gt $TIMEOUT ]; then
        echo "Timeout reached. host.k3d.internal address not found."
        exit 1
    fi
    CURRENT_ENTRIES=$(kubectl get configmap coredns -n kube-system -o yaml | yq -r '.data.NodeHosts' -)
    if echo "$CURRENT_ENTRIES" | grep -q "host.k3d.internal"; then
        HOST_IP_ADDRESS=$(echo "$CURRENT_ENTRIES" | grep "host.k3d.internal" | awk '{print $1}')
        FOUND=1
    else
        sleep 5
        SECONDS=$((SECONDS+5))
    fi
done

NEW_ENTRY="$HOST_IP_ADDRESS ${NEW_SKR_HOSTNAME}\n"

kubectl get configmap coredns -n kube-system -o yaml | \
  yq ".data.NodeHosts += \"${NEW_ENTRY}\"" - | \
  kubectl apply -f -

kubectl rollout restart -n kube-system deployment coredns
