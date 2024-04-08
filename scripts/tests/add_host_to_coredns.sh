#!/bin/bash

# Define custom hostname and IP address
HOSTNAME_TO_ADD="skr.cluster.local"
IP_ADDRESS="host.k3d.internal"

# Fetch the current CoreDNS ConfigMap
COREDNS_CONFIG_MAP=$(kubectl get configmap coredns -n kube-system -o json)

# Extract the NodeHosts content
NODEHOSTS=$(echo "$COREDNS_CONFIG_MAP" | jq -r '.data.NodeHosts')

# Add custom hostname to the NodeHosts content
NEW_NODEHOSTS_CONTENT="$NODEHOSTS $IP_ADDRESS $HOSTNAME_TO_ADD"

# Update the ConfigMap with the new NodeHosts content
kubectl patch configmap coredns -n kube-system --type=json -p="[{'op': 'replace', 'path': '/data/NodeHosts', 'value': \"$NEW_NODEHOSTS_CONTENT\"}]"

# Restart CoreDNS pods to apply the changes
kubectl rollout restart -n kube-system deployment coredns