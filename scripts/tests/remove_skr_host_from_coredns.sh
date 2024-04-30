#!/bin/bash

SKR_HOSTNAME="skr.cluster.local"

UPDATED_ENTRIES=$(kubectl get configmap coredns -n kube-system -o yaml | yq e '.data.NodeHosts' - | grep -v "$SKR_HOSTNAME")

kubectl get configmap coredns -n kube-system -o yaml | \
yq e ".data.NodeHosts = \"$UPDATED_ENTRIES\"" - | \
kubectl apply -f -

kubectl rollout restart -n kube-system deployment coredns