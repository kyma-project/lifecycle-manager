# Test watcher locally in two-cluster-setup using k3d

### KCP cluster setup

1. Create a local control-plane (KCP) cluster:
```shell
kyma provision k3d --name kcp-local --port 9080:80@loadbalancer \
--k3s-arg '--no-deploy=traefik@server:0'
```

2. Open `/etc/hosts` file on your local system:
```shell
sudo nano /etc/hosts
```
   Add entry for your local k3d registry created in the previous steps
```
127.0.0.1 k3d-kcp-local-registry.localhost
```

3. Install other pre-requisites required by the lifecycle-manager
    1. `Istio` CRDs using `istioctl`
```shell
brew install istioctl && \
istioctl install --set profile=demo -y
```
    2. `module-manager` CRDs
```shell
kubectl apply -f https://raw.githubusercontent.com/kyma-project/module-manager/main/config/crd/bases/operator.kyma-project.io_manifests.yaml
```

4. Deploy lifecycle-manager on the cluster:
```shell
make local-deploy-with-watcher IMG=eu.gcr.io/kyma-project/lifecycle-manager:latest
```

5. Create `module-template` by using [kyma-cli](https://github.com/kyma-project/cli)
   which serves the role of a component-descriptor for module installations.

   For this setup, we will create a module template from
   the [template-operator](https://github.com/kyma-project/template-operator) repository as reference. Adjust your path
   to your template-operator local directory or any other reference module operator accordingly.

```shell
kyma alpha create module -p ../template-operator --version 1.2.3 -w \
--registry k3d-kcp-local-registry.localhost:5001 --insecure
```
6. The previous step will create a `template.yaml` file in the root directory, which is the `module-template`, apply it
   to the cluster
```shell
kubectl apply -f template.yaml
```

### Create TLS secret

Run the following commands from your local directory for
the [kyma watcher repo](https://github.com/kyma-project/runtime-watcher) to generate PKI for the SKR webhook

1. Run the following commands to generate the PKI for the watcher web-hook
```shell
cd ../runtime-watcher/listener/example-mtls && \
cfssl gencert -initca ./kcp/gardener/cert_config/ca-csr.json | cfssljson -bare ca && \
cfssl gencert \
-ca=ca.pem \
-ca-key=ca-key.pem \
-config=./kcp/gardener/cert_config/ca-config.json \
-hostname="localhost,127.0.0.1,skr-webhook.kcp-system.svc.cluster.local,skr-webhook.kcp-system.svc" \
-profile=default \
./kcp/gardener/cert_config/ca-csr.json | cfssljson -bare signed-cert
```
2. Run the following command to create a secret holding the PKI for the watcher web-hook installation
```shell
cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: kcp-system-kyma-sample-skr-webhook-tls
  namespace: kcp-system
  labels:
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
data:
  ca.crt: $(cat ca.pem | base64 | tr -d '\n')
  tls.crt: $(cat signed-cert.pem | base64 | tr -d '\n')
  tls.key: $(cat signed-cert-key.pem | base64 | tr -d '\n')
EOF
```

### SKR cluster setup

Create a local kyma-runtime (SKR) cluster and a `kcp-system` namespace inside.

```shell
k3d cluster create skr-local && \
kubectl create namespace kcp-system
```

### Create Kyma CR and remote secret

1. Switch the context for using KCP cluster:
```shell
kubectl config use-context k3d-kcp-local
```
2. Generate and apply sample `Kyma CR` and its corresponding secret on KCP:
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
  config: $(k3d kubeconfig get skr-local | sed "s/0.0.0.0/host.k3d.internal/" | base64 | tr -d '\n')
---
apiVersion: operator.kyma-project.io/v1alpha1
kind: Kyma
metadata:
  name: kyma-sample
  namespace: kcp-system
spec:
  channel: regular
  sync:
    enabled: true
  modules:
  - name: template-operator
EOF
```

### Watcher installation verification

By checking the `Kyma CR` events, verify that the `SKRWebhookIsReady` ready condition is set to `True`

```yaml
Status:
  Conditions:
    Message: skrwebhook is synchronized
    Observed Generation: 1
    Reason: SKRWebhookIsReady
    Status: True
    Type: Ready
```

### Watcher event trigger

1. Switch the context for using SKR cluster
```shell
kubectl config use-context k3d-skr-local
```
2. Change the channel of the `template-operator` module to trigger a watcher event to KCP
```yaml
modules:
- name: template-operator
  channel: fast
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
2. By watching the lifecycle-manager logs, verify that the listener is logging messages indicating the reception of a
   message from the watcher
```log
{"level":"INFO","date":"2023-01-05T09:21:51.01093031Z","caller":"event/skr_events_listener.go:111","msg":"dispatched event object into channel","context":{"Module":"Listener","resource-name":"kyma-sample"}}
{"level":"INFO","date":"2023-01-05T09:21:51.010985Z","logger":"listener","caller":"controllers/setup.go:100","msg":"event coming from SKR, adding default/kyma-sample to queue","context":{}}                                                                            
{"level":"INFO","date":"2023-01-05T09:21:51.011080512Z","caller":"controllers/kyma_controller.go:87","msg":"reconciling modules","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"f9b42382-dc68-41d2-96de-02b24e3ac2d6"}}
{"level":"INFO","date":"2023-01-05T09:21:51.043800866Z","caller":"controllers/kyma_controller.go:206","msg":"syncing state","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"f9b42382-dc68-41d2-96de-02b24e3ac2d6","state":"Processing"}}
```

### Full-blown setup

For a full-blown setup please refer to the [comprehensive test setup documentation](creating-test-environment.md) and
complete the missing steps, e.g. deploying `module-manager`.

### Cleanup

Run the following command to remove the local testing clusters

```shell
k3d cluster rm kcp-local skr-local
```
