#!/usr/bin/env bash

k3d cluster list
echo "--- KLM DEPLOYMENT ---"
kubectl get deploy klm-controller-manager -n kcp-system -o yaml
kubectl describe deploy klm-controller-manager  -n kcp-system
echo "--- KLM POD ---"
kubectl describe pod -n kcp-system --selector=app.kubernetes.io/name=kcp-lifecycle-manager
echo "--- KLM LOGS ---"
kubectl logs deploy/klm-controller-manager -n kcp-system --container manager

set -e

kubectl config use-context k3d-skr
echo "--- SKR-WEBHOOK LOGS ---"
kubectl logs deploy/skr-webhook -n kyma-system --container server
echo "--- SKR-WEBHOOK METRICS ---"
SERVICE_NAME="skr-webhook-metrics"
NAMESPACE="kyma-system"
LOCAL_PORT=8080
REMOTE_PORT=2112

# Start port-forward in background
kubectl port-forward -n $NAMESPACE svc/$SERVICE_NAME $LOCAL_PORT:$REMOTE_PORT > /dev/null 2>&1 &
PF_PID=$!

# Wait for port-forward to become available
until curl -s https://localhost:$LOCAL_PORT/metrics > /dev/null; do
  sleep 0.5
done

# Fetch and print metrics
curl -s https://localhost:$LOCAL_PORT/metrics

# Clean up port-forward
kill $PF_PID
wait $PF_PID 2>/dev/null || true
