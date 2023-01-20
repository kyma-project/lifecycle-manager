# Test watcher locally in two-cluster-setup using K3D

The following steps provide you with a quick tour of how setup  the kyma-lifecycle-manager (short `KLM`) including the `Watcher` and a remote cluster. 

There are two ways shown: One with a very simple  KLM setup incl. watcher with security features turned off and another one for the KLM incl. watcher with security features (SAN-pinning and mTLS Communication) turned on. 

Depends on which guide you want to follow, skip the corresponding steps incl. the following tags: 
- *[SIMPLE]*
- *[SECURE]*

### Create KCP and SKR clusters
1. Run the following command to create a local control-plane (KCP) cluster:
```shell
k3d cluster create kcp-local --port 9080:80@loadbalancer \
--k3s-arg '--no-deploy=traefik@server:0'
```
2. Run the following command to create a local kyma-runtime (SKR) cluster:
```shell
k3d cluster create skr-local
```
### Switch context to the KCP cluster config
Run the following command to switch `kubectl` config context to prepare for running the subsequent commands:
```shell
kubectl config use-context k3d-kcp-local
```
### Install Istio and Manifest CRD
1. Run the following command to install `istioctl` using `brew`:
```shell
brew install istioctl
```
2. Run the following command to install Istio and Manifest CRD on the KCP cluster:
```shell
istioctl install --set profile=demo -y && \
kubectl apply -f https://raw.githubusercontent.com/kyma-project/module-manager/main/config/crd/bases/operator.kyma-project.io_manifests.yaml
```
### *[SECURE]* Install Cert-Manager
1. Run the following command to install the cert-manager by Jetstack:
```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
```

### Deploy lifecycle-manager with enabled Watcher on KCP:
Run the following command to deploy lifecycle-manager on the KCP cluster:

*[SIMPLE]*
```shell
make local-deploy-with-watcher IMG=eu.gcr.io/kyma-project/lifecycle-manager:latest
```

*[SECURE]*
```shell
make local-deploy-with-watcher-secured IMG=eu.gcr.io/kyma-project/lifecycle-manager:latest
```

### Apply sample module templates for sample-kyma:
Run the following commands to apply sample module templates needed for sample Kyma on the KCP cluster:
```shell
kubectl apply -f https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/config/samples/component-integration-installed/operator_v1alpha1_moduletemplate_kcp-module.yaml && \
kubectl apply -f https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/config/samples/component-integration-installed/operator_v1alpha1_moduletemplate_skr-module.yaml
```
### Create sample Kyma CR
Run the following command to generate and apply sample Kyma CR and its corresponding secret on KCP:
```shell
cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample
  namespace: kcp-system
  labels:
    "operator.kyma-project.io/kyma-name": "kyma-sample"
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
data:
  config: $(echo "${$(k3d kubeconfig get skr-local)/0.0.0.0/host.k3d.internal}" | base64)
---
apiVersion: operator.kyma-project.io/v1alpha1
kind: Kyma
metadata:
  annotations:
    skr-domain: "example.domain.com"
  name: kyma-sample
  namespace: kcp-system
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
### Verify logs
1. By watching the `skr-webhook` deployment's logs, verify that the KCP request is sent successfully
```log
1.6711877286771238e+09    INFO    skr-webhook    Kyma UPDATE validated from webhook 
1.6711879279507768e+09    INFO    skr-webhook    incoming admission review for: operator.kyma-project.io/v1alpha1, Kind=Kyma 
1.671187927950956e+09    INFO    skr-webhook    KCP    {"url": "http://host.k3d.internal:9080/v1/lifecycle-manager/event"} 
1.6711879280545895e+09    INFO    skr-webhook    sent request to KCP successfully for resource default/kyma-sample 
1.6711879280546305e+09    INFO    skr-webhook    kcp request succeeded
```
2. By watching the lifecycle-manager logs, verify that the listener is logging messages indicating the reception of a message from the watcher
```log
{"level":"INFO","date":"2023-01-05T09:21:51.01093031Z","caller":"event/skr_events_listener.go:111","msg":"dispatched event object into channel","context":{"Module":"Listener","resource-name":"kyma-sample"}}
{"level":"INFO","date":"2023-01-05T09:21:51.010985Z","logger":"listener","caller":"controllers/setup.go:100","msg":"event coming from SKR, adding default/kyma-sample to queue","context":{}}                                                                            
{"level":"INFO","date":"2023-01-05T09:21:51.011080512Z","caller":"controllers/kyma_controller.go:87","msg":"reconciling modules","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"f9b42382-dc68-41d2-96de-02b24e3ac2d6"}}
{"level":"INFO","date":"2023-01-05T09:21:51.043800866Z","caller":"controllers/kyma_controller.go:206","msg":"syncing state","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"f9b42382-dc68-41d2-96de-02b24e3ac2d6","state":"Processing"}}
```
### Cleanup
Run the following command to remove the local testing clusters
```shell
k3d cluster rm kcp-local skr-local
```
