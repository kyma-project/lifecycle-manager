# Test watcher locally in two-cluster-setup using K3D

### Contents

### Create KCP cluster
Run the following command to create a local control-plane (KCP) cluster:
```shell
k3d cluster create kcp-local --port 9080:80@loadbalancer \
--registry-create k3d-kyma-registry.localhost:0.0.0.0:5111 \
--k3s-arg '--no-deploy=traefik@server:0'
```

### Install Istio and Manifest CRD
Run the following command to install Istio and Manifest CRD on the KCP cluster:
```shell
kubectl config use-context k3d-kcp-local && \
istioctl install -y && \
kubectl apply -f https://raw.githubusercontent.com/kyma-project/module-manager/main/config/crd/bases/operator.kyma-project.io_manifests.yaml
```

### Build and push lifecycle-manager image
Run the following commands to build and push lifecycle-manager image to local k3d registry:
```shell
export K3D_REG=k3d-kyma-registry.localhost:5111 && \
make docker-build IMG=$K3D_REG/lifecycle-manager:latest && \
make docker-push IMG=$K3D_REG/lifecycle-manager:latest
```

### Deploy lifecycle-manager on KCP:
Run the following command to deploy lifecycle-manager on the KCP cluster:
```shell
kubectl config use-context k3d-kcp-local && \
export K3D_REG=k3d-kyma-registry.localhost:5111 && \
make local-deploy-with-watcher IMG=$K3D_REG/lifecycle-manager:latest
```

### Apply sample module templates for sample-kyma:
Run the following command to apply sample module templates needed for sample Kyma on the KCP cluster:
```shell
kubectl config use-context k3d-kcp-local && \
kubectl apply -f https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/config/samples/component-integration-installed/operator_v1alpha1_moduletemplate_kcp-module.yaml && \
kubectl apply -f https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/config/samples/component-integration-installed/operator_v1alpha1_moduletemplate_skr-module.yaml
```
### Apply sample Kyma CR
1. Run the following command to create a local kyma-runtime (SKR) cluster:
```shell
k3d cluster create skr-local
```
2. Run the following command to generate and apply sample Kyma CR and its corresponding secret on KCP:
```shell
kubectl config use-context k3d-kcp-local && \
cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample
  namespace: default
  labels:
    "operator.kyma-project.io/kyma-name": "kyma-sample"
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
data:
  config: $(echo "${$(k3d kubeconfig get skr-local)/0.0.0.0/host.k3d.internal}" | base64)
---
apiVersion: operator.kyma-project.io/v1alpha1
kind: Kyma
metadata:
  name: kyma-sample
  namespace: default
spec:
  channel: regular
  sync:
    enabled: true
  modules:
    - name: kcp-module
EOF
```
### Verify that watcher-webhook is installed on SKR
By checking the `Kyma CR` events, verify that the `SKRWebhookIsReady` ready condition is set to `True`.
```yaml
Status:                                              
    Conditions:                                        
        Message:               skrwebhook is synchronized
        Observed Generation:   1               
        Reason:                SKRWebhookIsReady
        Status:                True
        Type:                  Ready
```
### Edit kyma on SKR
1. Run the following command to switch the context to using the SKR cluster
```shell
kubectl config use-context k3d-skr-local
```
2. Add the following line to the Kyma CR `spec` to trigger the watcher KCP update
```yaml
  modules:
  - name: skr-module
```
### verify lifecycle-manager logs
By watching the lifecycle-manager logs, verify that the listener is logging messages indicating the reception of a message from the watcher
```log
{"level":"INFO","date":"2022-12-16T10:52:07.983431734Z","logger":"request–verifier","caller":"security/san_pinning.go:85","msg":"###### Request Header [By=spiffe://cluster.local/ns/kcp-system/sa/klm-controller-manager;Hash=d28f48b3777e8431cdfce5f36da0e1b1f239de7c9c2ec3313ef163f3918610a0;Subject=\"\";URI=spiffe://cluster.local/ns/istio-system/sa/istio-ingressgateway-service-account]","context":{}} 
{"level":"INFO","date":"2022-12-16T10:52:07.98353109Z","caller":"event/skr_events_listener.go:103","msg":"request could not be verified - Event will not be dispatched","context":{"Module":"Listener","error":"empty certificate"}}       
{"level":"INFO","date":"2022-12-16T10:52:13.406499949Z","caller":"controllers/kyma_controller.go:87","msg":"reconciling modules","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"64add714-e3fb-4513-827d-9bb9ba2325a3"}}                                                                                          
{"level":"INFO","date":"2022-12-16T10:52:13.451737025Z","caller":"controllers/kyma_controller.go:206","msg":"syncing state","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"64add714-e3fb-4513-827d-9bb9ba2325a3","state":"Processing"}}
```
### verify SKR-watcher logs
By watching the `skr-webhook` deployment's logs, verify that the KCP request is sent successfully
```log
1.6711877286771238e+09    INFO    skr-webhook    Kyma UPDATE validated from webhook 
1.6711879279507768e+09    INFO    skr-webhook    incoming admission review for: operator.kyma-project.io/v1alpha1, Kind=Kyma 
1.671187927950956e+09    INFO    skr-webhook    KCP    {"url": "http://host.k3d.internal:9080/v1/lifecycle-manager/event"} 
1.6711879280545895e+09    INFO    skr-webhook    sent request to KCP successfully for resource default/kyma-sample 
1.6711879280546305e+09    INFO    skr-webhook    kcp request succeeded
```
### cleanup
1. Run the following command to remove the local testing clusters
```shell
k3d cluster rm kcp-local skr-local
```
2. Run the following commands to remove the kubeconfigs of the local testing clusters:
```shell
kubectl config unset users.admin@k3d-skr-cluster && \
kubectl config unset users.admin@k3d-kcp-cluster && \
kubectl config unset clusters.k3d-skr-cluster && \
kubectl config unset clusters.k3d-kcp-cluster && \
kubectl config unset contexts.k3d-skr-cluster && \
kubectl config unset contexts.k3d-kcp-cluster
```
