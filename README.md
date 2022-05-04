# kyma-operator


## Webhook Installation

```shell
make docker-build IMG=op:v0.0.5-test-deploy 
kind load docker-image op:v0.0.5-test-deploy --name kind 
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
kubectl apply -f config/samples/component-integration-deployed/rbac
kubectl apply -f https://raw.githubusercontent.com/Tomasz-Smelcerz-SAP/kyma-operator-istio/main/k8s-api/config/crd/bases/kyma.kyma-project.io_istioconfigurations.yaml
make deploy IMG=op:v0.0.5-test-deploy 
kubectl apply -f config/samples/component-integration-deployed
```