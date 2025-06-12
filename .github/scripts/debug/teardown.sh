#!/usr/bin/env bash

k3d cluster list
echo "--- KLM DEPLOYMENT ---"
kubectl get deploy klm-controller-manager -n kcp-system -o yaml
kubectl describe deploy klm-controller-manager  -n kcp-system
echo "--- KLM POD ---"
kubectl describe pod -n kcp-system --selector=app.kubernetes.io/name=kcp-lifecycle-manager
echo "--- KLM LOGS ---"
kubectl logs deploy/klm-controller-manager -n kcp-system --container manager

kubectl config use-context k3d-skr
echo "--- SKR-WEBHOOK LOGS ---"
kubectl logs deploy/skr-webhook -n kyma-system --container server
echo "--- SKR-WEBHOOK METRICS ---"
kubectl port-forward svc/skr-webhook-metrics 8080:2112 -n kyma-system
curl -s http://localhost:8080/metrics
