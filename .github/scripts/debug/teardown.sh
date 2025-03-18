#!/usr/bin/env bash

k3d cluster list
echo "--- KLM DEPLOYMENT ---"
kubectl get deploy klm-controller-manager -n kcp-system -o yaml
kubectl describe deploy klm-controller-manager  -n kcp-system
echo "--- KLM POD ---"
kubectl describe pod -n kcp-system --selector=app.kubernetes.io/name=kcp-lifecycle-manager
echo "--- KLM LOGS ---"
kubectl logs deploy/klm-controller-manager -n kcp-system --container manager
