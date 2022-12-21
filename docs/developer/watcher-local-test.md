# Test watcher locally in two-cluster-setup using K3D

### Contents

### Export customizable attributes
```shell
export KLM_PATH=<local-path-of-kyma-lifecycle-manager-project>
export KLM_VERSION=<docker-image-tag-for-local-testing>
export KUBE_CONFIG_DIR=<path-to-export-local-kubeconfigs-to>
```

### Create KCP cluster

Run the following command to create a local control-plane (KCP) cluster:
```shell
k3d cluster create kcp-local --port 9080:80@loadbalancer \
--registry-create k3d-registry.localhost:0.0.0.0:5111 \
--k3s-arg '--no-deploy=traefik@server:0'
```

### Create SKR cluster
Run the following command to create a local kyma-runtime (SKR) cluster:
```shell
k3d cluster create skr-local
```
### Install Istio and Manifest CRD
Run the following command to install Istio and Manifest CRD on the KCP cluster:
```shell
istioctl install -y && \
kubectl apply -f https://raw.githubusercontent.com/kyma-project/module-manager/main/config/crd/bases/operator.kyma-project.io_manifests.yaml
```

### Build and push lifecycle-manager image
1. Run the following command to get KCP cluster kubeconfig and export it:
```shell
k3d kubeconfig get kcp-local > $KUBE_CONFIG_DIR/kcp-local.yaml && \
export KUBECONFIG=$KUBE_CONFIG_DIR/kcp-local.yaml
```
2. Run the following command to build and push lifecycle-manager image to local k3d registry:
```shell
export K3D_REG=k3d-registry.localhost:5111
make -C $KLM_PATH docker-build IMG=$K3D_REG/lifecycle-manager:$KLM_VERSION && \
make -C $KLM_PATH docker-push IMG=$K3D_REG/lifecycle-manager:$KLM_VERSION
```

### Deploy lifecycle-manager on KCP:
Run the following command to deploy lifecycle-manager on the KCP cluster:
```shell
make -C $KLM_PATH install && \
make -C $KLM_PATH local-deploy-with-watcher IMG=$K3D_REG/lifecycle-manager:$KLM_VERSION
```

### Apply sample module templates for sample-kyma:
Run the following command to deploy lifecycle-manager on the KCP cluster:
```shell
kubectl apply -f $KLM_PATH/config/samples/component-integration-installed/operator_v1alpha1_moduletemplate_kcp-module.yaml && \
kubectl apply -f $KLM_PATH/config/samples/component-integration-installed/operator_v1alpha1_moduletemplate_skr-module.yaml
```
### Apply sample Kyma CR
1. Run the following command to get SKR cluster kubeconfig and export it:
```shell
k3d kubeconfig get skr-local > $KUBE_CONFIG_DIR/skr-local.yaml && \
export KUBECONFIG=$KUBE_CONFIG_DIR/skr-local.yaml
```
2. Run the following command to generate and apply sample Kyma CR and its corresponding secret:
```shell
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
By checking the Kyma Object events, verify that the `SKRWebhookIsReady` ready condition has a `True` Status. 
```yaml
Status:                                              
    Conditions:                                        
        Message:               skrwebhook is synchronized
        Observed Generation:   1               
        Reason:                SKRWebhookIsReady
        Status:                True
        Type:                  Ready
        Last Transition Time:  2022-12-16T10:45:24Z
```
### Edit kyma on SKR
By adding the following line to the Kyma CR `spec`, trigger the watcher KCP update
```yaml
  modules:
  - name: skr-module
```
### verify lifecycle-manager logs
By watching the lifecycle-manager logs, verify that the listener is logging messages indicating the reception of a message from the watcher
```log
{"level":"INFO","date":"2022-12-16T10:52:07.983431734Z","logger":"request–verifier","caller":"security/san_pinning.go:85","msg":"###### Request Header [By=spiffe://cluster.local/ns/kcp-system/sa/klm-controller-manager;Hash=d28f48b3777e8431cdfce5f36da0e1b1f239de7c9c2ec3313ef163f3918610a0;Subject=\"\";URI=spiffe://cluster.local/ns/istio-system/sa/istio-ingressgateway-service-account]","context":{}}                                                                      │
{"level":"INFO","date":"2022-12-16T10:52:07.98353109Z","caller":"event/skr_events_listener.go:103","msg":"request could not be verified - Event will not be dispatched","context":{"Module":"Listener","error":"empty certificate"}}       
{"level":"INFO","date":"2022-12-16T10:52:13.406499949Z","caller":"controllers/kyma_controller.go:87","msg":"reconciling modules","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"64add714-e3fb-4513-827d-9bb9ba2325a3"}}                                                                                          
{"level":"INFO","date":"2022-12-16T10:52:13.451737025Z","caller":"controllers/kyma_controller.go:206","msg":"syncing state","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"64add714-e3fb-4513-827d-9bb9ba2325a3","state":"Processing"}}
```
### verify SKR-watcher logs
By watching the `skr-webhook` deployment's logs, verify that the KCP request is sent successfully
```log
1.6711877286771238e+09    INFO    skr-webhook    Kyma UPDATE validated from webhook                                                                                  │
1.6711879279507768e+09    INFO    skr-webhook    incoming admission review for: operator.kyma-project.io/v1alpha1, Kind=Kyma                                         │
1.671187927950956e+09    INFO    skr-webhook    KCP    {"url": "http://host.k3d.internal:9080/v1/lifecycle-manager/event"}                                           │
1.6711879280545895e+09    INFO    skr-webhook    sent request to KCP successfully for resource default/kyma-sample                                                   │
1.6711879280546305e+09    INFO    skr-webhook    kcp request succeeded
```
