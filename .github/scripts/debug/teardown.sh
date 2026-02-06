#!/usr/bin/env bash
set +e

kubectl config use-context k3d-kcp

k3d cluster list
echo "### KCP ModuleTemplate ###"
kubectl get moduletemplate -n kcp-system -o wide
kubectl get moduletemplate -n kcp-system -o yaml

echo "### KCP ModuleReleaseMeta ###"
kubectl get modulereleasemeta -n kcp-system -o wide
kubectl get modulereleasemeta -n kcp-system -o yaml

echo "### KCP Kyma ###"
kubectl get kyma -n kcp-system -o wide
kubectl get kyma -n kcp-system -o yaml

echo "### KCP Manifest ###"
kubectl get manifest -n kcp-system -o wide
kubectl get manifest -n kcp-system -o yaml

echo "### KLM DEPLOYMENT ###"
kubectl get deploy klm-controller-manager -n kcp-system -o yaml
kubectl describe deploy klm-controller-manager  -n kcp-system
echo "### KLM POD ###"
kubectl describe pod -n kcp-system --selector=app.kubernetes.io/name=kcp-lifecycle-manager
echo "### KLM LOGS ###"
kubectl logs deploy/klm-controller-manager -n kcp-system --container manager

kubectl config use-context k3d-skr

echo "### SKR DEPLOYMENT OVERVIEW ###"
kubectl get deploy -A -o wide

echo "### SKR DESCRIBE PODS with template-operator IMAGE ###"

for entry in $(kubectl get pods --all-namespaces -o json | jq -r \
    '.items[]
      | select(.spec.containers[].image
      | contains("europe-docker.pkg.dev/kyma-project/prod/template-operator"))
      | "\(.metadata.namespace):\(.metadata.name)"');
do kubectl describe pod -n $(echo -n $entry | sed 's/:/ /'); echo "##################==";
done

echo "### SKR-WEBHOOK POD ###"
kubectl describe deploy/skr-webhook  -n kyma-system
kubectl get pods -l app=skr-webhook -n kyma-system -o wide

echo "### SKR-WEBHOOK LOGS ###"
kubectl logs deploy/skr-webhook -n kyma-system --container server

