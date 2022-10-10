# Set up the test environment

### Contents
* [Provision clusters and OCI registry](#provision-clusters-and-oci-registry)
* [Create module operator and bundle module](#create-module-operator-and-bundle-module)
* [Install Kyma and run lifecycle-manager operator](#install-kyma-and-run-lifecycle-manager-operator)
* [Verify installation](#verify-installation)

### Provision clusters and OCI registry

You can choose between either a single-cluster or a two-cluster setup.

- In a **_single cluster setup_**, provision one cluster - since both control-plane (KCP) and Kyma runtime (SKR) are served from one cluster.

- In a **_dual cluster setup_**, provision two clusters - one as control-plane (KCP) and the second as runtime (SKR).

- Refer [cluster and registry setup](provision-cluster-and-registry.md) for more information.

### Create module operator and bundle module

To bundle your module image and operator, please refer to the detailed information inside [template-operator docs](../../samples/template-operator/README.md#bundling-and-installation).

### Install Kyma and run lifecycle-manager operator

1. If you're using the two-cluster setup, set your `KUBECONFIG` to the KCP Cluster context.

2. Create the `kyma-system` Namespace:

   ```sh
   kubectl create ns kyma-system
   ```

3. Create a Secret to access the cluster which acts as SKR:

   Go to `https://github.com/kyma-project/lifecycle-manager` and run the following commands, to create a secret and apply it to the KCP Cluster:


   ```sh
   chmod 755 ./operator/config/samples/secret/k3d-secret-gen.sh
   ./operator/config/samples/secret/k3d-secret-gen.sh
   ```

   > _**NOTE:**_ In the **single-cluster setup**, adjust your contexts for applying the secret using `KCP_CLUSTER_CTX` and `SKR_CLUSTER_CTX` - both should point to the same cluster.

4. To install the Module Manager CRDs, check out `https://github.com/kyma-project/module-manager`, navigate to the operator `cd operator`, and run `make install`.

5. Run [module-manager](https://github.com/kyma-project/module-manager/tree/main/operator) and [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator) in this order.
   * local: run following commands against your cluster's kubeconfig
   ```makefile
    make install
    make run
   ```
   * in-cluster
   ```makefile
   # for local registry adjust IMG value accordingly
   # using remote registry (replace `PR-73` with your desired tag)
   make deploy IMG=eu.gcr.io/kyma-project/module-manager:PR-73
   ```

   > _**NOTE:**_ If you get messages like `no matches for kind "VirtualService" in version "networking.istio.io/v1beta1"`, don't worry. This is normal if you install the operators in a cluster without a certain dependency. If you do not need this dependency for your test, you can safely ignore the warning.

6. Start the Kyma installation.

    >_**NOTE:**_ for using Google Artifact Registry: if you use gcloud cli, make sure your local docker config (`~/.docker/config.json`) does not contains `gcloud` `credHelpers` entry, (e.g: `"europe-west3-docker.pkg.dev": "gcloud",`), otherwise this might cause authentication issue for module-manager while fetching remote oci image layer.

   1. Make sure required module templates prepared as part of [Bundle your module](#create-module-operator-and-bundle-module) are pushed to the KCP cluster. 

   2. To create a request for Kyma installation of the module in `samples/template-operator` of the Lifecycle Manager, run:

      ```sh
      sh hack/gen-kyma.sh

      # for a single cluster setup set .spec.sync.enabled = false
      # in two-cluster setup, use this command:
      kubectl apply -f kyma.yaml
      ```

   3. Check the progress of your Kyma installation with:
      ```sh
      kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P
      ```
      You should get results like this:
      ```yaml
             conditions:
               - lastTransitionTime: "2022-08-18T18:10:09Z"
                 message: module is Ready
                 reason: template
                 status: "True"
                 type: Ready
             moduleInfos:
               - moduleName: template
                 name: templatekyma-sample
                 namespace: kyma-system
                 templateInfo:
                   channel: stable
                   generation: 1
                   gvk:
                     group: operator.kyma-project.io
                     kind: Manifest
                     version: v1alpha1
             observedGeneration: 1
             state: Ready
             ```

### Verify installation

- To observe the installation in the runtime, switch the context to the SKR context, and verify the status.

  ```sh
  kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P
  ```

  If you get `.status.state: Ready`, the installation succeeded.
