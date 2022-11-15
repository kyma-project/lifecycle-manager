# Set up test environment for local testing

### Contents
* [Provision clusters and OCI registry](#provision-clusters-and-oci-registry)
* [Create module operator and bundle module](#create-module-operator-and-bundle-module)
* [Create Kyma custom resource](#create-kyma-custom-resource)
* [Verify installation](#verify-installation)

### Provision clusters and OCI registry

You can choose between either a single-cluster or a two-cluster setup.

- In a **_single cluster setup_**, provision one cluster - since both control-plane (KCP) and Kyma runtime (SKR) are served from one cluster.

- In a **_dual cluster setup_**, provision two clusters - one as control-plane (KCP) and the second as runtime (SKR).

- Refer [cluster and registry setup](provision-cluster-and-registry.md) for more information.

### Create module operator and bundle module

To bundle your module image and operator, please refer to the detailed information inside [template-operator docs](../../../template-operator/README.md#bundling-and-installation).

### Create Kyma custom resource

1. If you're using the two-cluster setup, set your `KUBECONFIG` to the KCP Cluster context.
   Next, create a `Secret` to access the SKR cluster from KCP:

   Go to `https://github.com/kyma-project/lifecycle-manager` and run the following commands, to create a secret and apply it to the KCP Cluster:

   ```sh
   chmod 755 ./config/samples/secret/k3d-secret-gen.sh
   ./config/samples/secret/k3d-secret-gen.sh
   ```

2. Create the `kyma-system` Namespace:

3. Run [module-manager](https://github.com/kyma-project/module-manager/tree/main/operator) and [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main) in this order.
   * local: run following commands against your cluster's kubeconfig
   ```shell
    make install
    make run
   ```
   * in-cluster
   ```shell
   # for local registry adjust IMG value accordingly
   # using remote registry (replace `latest` with your desired tag)
   make deploy IMG=eu.gcr.io/kyma-project/module-manager:latest
   ```

   > _**NOTE:**_ Ignore dependency errors like `no matches for kind "VirtualService" in version "networking.istio.io/v1beta1"`, if it is irrelevant for your test setup.
   For using Google Artifact Registry: if you use `gcloud` cli, make sure your local docker config (`~/.docker/config.json`) does not contains `gcloud` `credHelpers` entry, (e.g: `"europe-west3-docker.pkg.dev": "gcloud",`), otherwise this might cause authentication issue for module-manager while fetching remote oci image layer.

4. Start the Kyma installation.

   1. Make sure required module templates are prepared and pushed to the KCP cluster, as described in the [bundle your module](#create-module-operator-and-bundle-module) section. 

   2. Create a Kyma custom resource specifying the module corresponding to the label value `operator.kyma-project.io/module-name"` of your Module Template.
   
      ```sh
      # for a single cluster setup set .spec.sync.enabled = false
      cat <<EOF | kubectl apply -f -
      apiVersion: operator.kyma-project.io/v1alpha1
      kind: Kyma
      metadata:
         name: my-kyma
         namespace: kyma-system
      spec:
         sync:
            enabled: true
         channel: stable
         modules:
         - name: sample
      EOF 
      ```
   
   3. Check the progress of your Kyma CR installation. The success indicator `.status.state` should be set to `Ready`.
      ```sh
       kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P
      ```
      

### Verify installation

To observe the installation in the runtime, switch the context to the SKR context, and verify the status.

  ```sh
  kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P
  ```
  The success indicator `.status.state` should be set to `Ready`, consistent with the KCP state.
