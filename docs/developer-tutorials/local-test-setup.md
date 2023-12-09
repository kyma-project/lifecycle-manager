# Local test setup in the control-plane mode using k3d

## Context

This tutorial shows how to configure a fully working e2e test setup including the following components:

- Lifecycle Manager
- Runtime Watcher on a remote cluster
- `template-operator` on a remote cluster as an example

This setup is deployed with the following security features enabled:

- Strict mTLS connection between Kyma Control Plane (KCP) and SKR clusters
- SAN Pinning (SAN of client TLS certificate needs to match the DNS annotation of a corresponding Kyma CR)

> **NOTE:** If you want to use remote clusters instead of a local k3d setup or external registries, please refer to the following guides for the cluster and registry setup:
>
> - [Provision cluster and OCI registry](provision-cluster-and-registry.md)
> - [Create a test environment on Google Container Registry (GCR)](prepare-gcr-registry.md)

## Procedure

### KCP cluster setup

1. Create a local KCP cluster:

    ```shell
    k3d cluster create kcp-local --port 9443:443@loadbalancer \
    --registry-create k3d-registry.localhost:0.0.0.0:5111 \
    --k3s-arg '--disable=traefik@server:0'
    ```

2. Open `/etc/hosts` file on your local system:

   ```shell
   sudo nano /etc/hosts
   ```

   Add an entry for your local k3d registry created in step 1:

   ```txt
   127.0.0.1 k3d-registry.localhost
   ```

3. Install the following prerequisites required by Lifecycle Manager:

   1. Istio CRDs using `istioctl`:

      ```shell
      brew install istioctl && \
      istioctl install --set profile=demo -y
      ```

   2. `cert-manager` by Jetstack:

       ```shell
       kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.3/cert-manager.yaml
       ```

4. Deploy Lifecycle Manager on the cluster:

    ```shell
    make local-deploy-with-watcher IMG=europe-docker.pkg.dev/kyma-project/prod/lifecycle-manager:latest
    ```

   > **TIP:** If you get an error similar to the following, wait a couple of seconds and rerun the command.
   >
   > ```shell
   > Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "webhook.cert-manager.io": failed to call webhook: Post "https://cert-manager-webhook.cert-manager.svc:443/mutate?timeout=10s": no endpoints available for service "cert-manager-webhook"
   > ```

   <details>
      <summary>Custom Lifecycle Manager image deployment</summary>
      If you want to test a custom image of Lifecycle Manager, adapt the `IMG` variable in `Makefile` and run the following:

      ```shell
      make docker-build
      make docker-push
      make local-deploy-with-watcher IMG=<image-name>:<image-tag>
      ```

   </details>

5. Create a ModuleTemplate CR using [Kyma CLI](https://github.com/kyma-project/cli).
   The ModuleTemplate CR includes component descriptors for module installations.

   In this tutorial, we will create a ModuleTemplate CR from the [`template-operator`](https://github.com/kyma-project/template-operator) repository.
   Adjust the path to your `template-operator` local directory or any other reference module operator accordingly.

   ```shell
   kyma alpha create module -p ../template-operator --version 1.2.3 \
   --registry k3d-registry.localhost:5111 --insecure --module-config-file ../template-operator/module-config.yaml
   ```

6. Verify images pushed to the local registry:

   ```shell
   curl http://k3d-registry.localhost:5111/v2/_catalog\?n\=100
   ```

   The output should look like the following:

   ```shell
   {"repositories":["component-descriptors/kyma-project.io/template-operator"]}
   ```

7. Open the generated `template.yaml` file and change the following line:

   ```yaml
    <...>
      - baseUrl: k3d-registry.localhost:5111
    <...>
   ```

   To the following:

    ```yaml
    <...>
      - baseUrl: k3d-registry.localhost:5000
    <...>
   ```

   You need the change because the operators are running inside of two local k3d clusters, and the internal port for the k3d registry is set by default to `5000`.

8. Apply the template:

   ```shell
   kubectl apply -f template.yaml
   ```

### SKR cluster setup

Create a local Kyma runtime (SKR) cluster:

```shell
k3d cluster create skr-local
```

### Create a Kyma CR and a remote Secret

1. Switch the context for using the KCP cluster:

    ```shell
    kubectl config use-context k3d-kcp-local
    ```

2. Generate and apply a sample Kyma CR and its corresponding Secret on KCP:

    ```shell
    cat <<EOF | kubectl apply -f -
   apiVersion: v1
   kind: Secret
   metadata:
      name: kyma-sample
      namespace: kcp-system
      labels:
        "operator.kyma-project.io/kyma-name": "kyma-sample"
        "operator.kyma-project.io/managed-by": "lifecycle-manager"
   data:
      config: $(k3d kubeconfig get skr-local | sed 's/0\.0\.0\.0/host.k3d.internal/' | base64 | tr -d '\n')
   ---
   apiVersion: operator.kyma-project.io/v1beta2
   kind: Kyma
   metadata:
      annotations:
        skr-domain: "example.domain.com"
      name: kyma-sample
      namespace: kcp-system
   spec:
      channel: regular
   EOF
    ```

   <details>
      <summary>Running Lifecycle Manager on a local machine and not on a cluster</summary>
      If you are running Lifecycle Manager on your local machine and not as a deployment on a cluster, use the following to create a Kyma CR and Secret:

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
         config: $(k3d kubeconfig get skr-local | base64 | tr -d '\n')
      ---
      apiVersion: operator.kyma-project.io/v1beta1
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
         - name: template-operator
      EOF
   ```

   </details>

### Watcher and module installation verification

Check the Kyma CR events to verify if the `SKRWebhookIsReady` condition is set to `True`.
Also make sure if the state of the `template-operator` is `Ready` and check the overall `state`.

```yaml
status:
   activeChannel: regular
   conditions:
      - lastTransitionTime: "2023-02-28T06:42:00Z"
        message: skrwebhook is synchronized
        observedGeneration: 1
        reason: SKRWebhookIsReady
        status: "True"
        type: Ready
   lastOperation:
      lastUpdateTime: "2023-02-28T06:42:00Z"
      operation: kyma is ready
   modules:
      - channel: regular
        fqdn: kyma-project.io/template-operator
        manifest:
           apiVersion: operator.kyma-project.io/v1beta1
           kind: Manifest
           metadata:
              generation: 1
              name: kyma-sample-template-operator-3685142144
              namespace: kcp-system
        name: template-operator
        state: Ready
        template:
           apiVersion: operator.kyma-project.io/v1beta1
           kind: ModuleTemplate
           metadata:
              generation: 1
              name: moduletemplate-template-operator
              namespace: kcp-system
        version: 1.2.3
   state: Ready
```

### (Optional) Check the functionality of the Watcher component

1. Switch the context to use the SKR cluster:

    ```shell
    kubectl config use-context k3d-skr-local
    ```

2. Change the channel of the `template-operator` module to trigger a watcher event to KCP:

    ```yaml
      modules:
      - name: template-operator
        channel: fast
    ```

### Verify logs

1. By watching the `skr-webhook` deployment logs, verify if the KCP request is sent successfully:

    ```log
    1.6711877286771238e+09    INFO    skr-webhook    Kyma UPDATE validated from webhook 
    1.6711879279507768e+09    INFO    skr-webhook    incoming admission review for: operator.kyma-project.io/v1alpha1, Kind=Kyma 
    1.671187927950956e+09    INFO    skr-webhook    KCP    {"url": "https://host.k3d.internal:9443/v1/lifecycle-manager/event"} 
    1.6711879280545895e+09    INFO    skr-webhook    sent request to KCP successfully for resource default/kyma-sample 
    1.6711879280546305e+09    INFO    skr-webhook    kcp request succeeded
    ```

2. In Lifecycle Manager's logs, verify if the listener is logging messages indicating the reception of a message from the watcher:

    ```log
    {"level":"INFO","date":"2023-01-05T09:21:51.01093031Z","caller":"event/skr_events_listener.go:111","msg":"dispatched event object into channel","context":{"Module":"Listener","resource-name":"kyma-sample"}}
    {"level":"INFO","date":"2023-01-05T09:21:51.010985Z","logger":"listener","caller":"controllers/setup.go:100","msg":"event coming from SKR, adding default/kyma-sample to queue","context":{}}                                                                            
    {"level":"INFO","date":"2023-01-05T09:21:51.011080512Z","caller":"controllers/kyma_controller.go:87","msg":"reconciling modules","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"f9b42382-dc68-41d2-96de-02b24e3ac2d6"}}
    {"level":"INFO","date":"2023-01-05T09:21:51.043800866Z","caller":"controllers/kyma_controller.go:206","msg":"syncing state","context":{"controller":"kyma","controllerGroup":"operator.kyma-project.io","controllerKind":"Kyma","kyma":{"name":"kyma-sample","namespace":"default"},"namespace":"default","name":"kyma-sample","reconcileID":"f9b42382-dc68-41d2-96de-02b24e3ac2d6","state":"Processing"}}
    ```

### Cleanup

Run the following command to remove the local testing clusters:

```shell
k3d cluster rm kcp-local skr-local
```
