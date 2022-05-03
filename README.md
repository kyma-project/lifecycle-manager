# kyma-operator


## Webhook Installation

```shell
make docker-build IMG=op:v0.0.3-test-deploy 
kind load docker-image op:v0.0.3-test-deploy --name kind 
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
make deploy IMG=op:v0.0.3-test-deploy 
```