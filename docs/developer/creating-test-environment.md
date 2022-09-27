# Set up the test environment

## Instructions

### Create the cluster(s) for testing your Kyma module

You can choose between a single-cluster setup or a two-cluster setup.

- For testing with a single cluster that acts as both control-plane (KCP) and Kyma runtime (SKR), follow the [single cluster setup guide](creating-test-environment-singlecluster.md).


- For testing with two clusters (with two registries), one acting as the KCP and another as the SKR cluster, follow the [two-cluster setup guide](creating-test-environment-twocluster.md).

### Bundle your module

To bundle your module image and operator, please refer to the detailed information inside [template-operator docs](https://github.com/kyma-project/lifecycle-manager/blob/main/samples/template-operator/README.md#bundling-and-installation).

### Install Kyma with your module

1. If you're using the two-cluster setup, set your `KUBECONFIG` to the KCP Cluster context.

2. Create the `kyma-system` Namespace:

   ```sh
   kubectl create ns kyma-system
   ```

3. Create a Secret to access the cluster which acts as SKR:

   Go to `https://github.com/kyma-project/lifecycle-manager` and run the following commands:

   ```sh
   chmod 755 ./operator/config/samples/secret/k3d-secret-gen.sh
   ./operator/config/samples/secret/k3d-secret-gen.sh
   ```

   > _**NOTE:**_ In the two-cluster setup, adjust your contexts for applying the secret using KCP_CLUSTER_CTX and SKR_CLUSTER_CTX.

4. To install the Module Manager CRDs, check out `https://github.com/kyma-project/module-manager`, navigate to the operator `cd operator`, and run `make install`.

5. Run [module-manager](https://github.com/kyma-project/module-manager/operator) and [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/operator) in this order.
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

   1. Make sure required module templates prepared as part of [Bundle your module](#bundle-your-module) are pushed to the KCP cluster. 

   2. To create a request for Kyma installation of the module in `samples/template-operator` of the Lifecycle Manager, run:

      ```sh
      sh hack/gen-kyma.sh

      # for a single cluster setup set .spec.sync.enabled = false
      # in two-cluster setup, use this command:
      kubectl apply -f kyma.yaml
      ```

   3. Check the progress of your Kyma installation; for example, with `kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P`. You should get a result like this:

### Verify the installation

- To observe the installation in the runtime, switch the context to the SKR context, and verify the status.

  ```sh
  kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P
  ```

  If you get `.status.state: Ready`, the installation succeeded.
