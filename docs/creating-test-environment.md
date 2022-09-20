# Set up the test environment

## Instructions

### Create the cluster(s) for testing your Kyma module

You can choose between a single-cluster setup or a two-cluster setup.


- For testing with a single cluster that acts as both control-plane (KCP) and Kyma runtime (SKR), follow the [single cluster setup guide](creating-test-environment-singlecluster.md).


- For testing with two clusters (with two registries), one acting as the KCP and another as the SKR cluster, follow the [two-cluster setup guide](creating-test-environment-twocluster.md).

### Build your module

To demonstrate how the bundling of a Kyma module works, the following example uses our reference implementation for a reconciliation operator; see the [`template-operator` in Github](https://github.com/kyma-project/lifecycle-manager/tree/main/samples/template-operator)).

1. Go to your operator folder:

   In `https://github.com/kyma-project/lifecycle-manager`, go to `cd samples/template-operator`.

2. Generate and push the module image of the operator and the charts:


   ```sh
   make module-operator-chart module-image
   ```

### Install Kyma with your module

1. If you're using the two-cluster setup, set your `KUBECONFIG` to the KCP Cluster context.


2. Create the `kyma-system` Namespace:

   ```sh
   kubectl create ns kyma-system

Create a Secret to access the cluster which acts as SKR:

   Go to `https://github.com/kyma-project/lifecycle-manager` and run the following commands:

   ```sh
   chmod 755 ./operator/config/samples/secret/k3d-secret-gen.sh
   ./operator/config/samples/secret/k3d-secret-gen.sh

   > **NOTE:** In the two-cluster setup, adjust your contexts for applying the secret using KCP_CLUSTER_CTX and SKR_CLUSTER_CTX.

4. Build and push the module to the registry.

   In `https://github.com/kyma-project/lifecycle-manager`, switch to subfolder `samples/template-operator` and run `make module-build`.


   > **TIP:** If you use a remote registry and you receive a `403` or `401` error, maybe your credentials timed out. To fix this, recreate the `MODULE_CREDENTIALS` variable.


_Note for two cluster mode: set your `KUBECONFIG` to the KCP Cluster context_

### 3.3.1 Install Module Manager CRDs

1. Checkout https://github.com/kyma-project/module-manager and navigate to the operator: `cd operator`

2. Run the Installation Command

   ```sh
   make install
   ```

### 3.3.2 Install Lifecycle Manager CRDs

1. Checkout https://github.com/kyma-project/lifecycle-manager and navigate to the operator: `cd operator`

2. Run the Installation Command

   ```sh
   make install
   ```

Ensure the CRDs are installed with `kubectl get crds | grep kyma-project.io`:

```
manifests.component.kyma-project.io        2022-08-18T16:27:21Z
kymas.operator.kyma-project.io             2022-08-18T16:29:28Z
moduletemplates.operator.kyma-project.io   2022-08-18T16:29:28Z
```

### 3.3.3 Run the operators

#### 3.3.3.1 Deploy and run operators in cluster

_Note: The order of installation is important due to cross-dependencies in CRDs_

In https://github.com/kyma-project/module-manager/operator run

```sh
# using local registry
make docker-build docker-push deploy IMG=$IMG_REGISTRY/module-manager:dev

# using remote registry (replace `PR-73` with your desired tag)
make deploy IMG=eu.gcr.io/kyma-project/module-manager:PR-73
```

In https://github.com/kyma-project/lifecycle-manager/operator run

```sh
# using local registry
make docker-build docker-push deploy IMG=$IMG_REGISTRY/lifecycle-manager:dev

# using remote registry (replace `PR-122` with your desired tag)
make deploy IMG=eu.gcr.io/kyma-project/lifecycle-manager:PR-122
```

_Note: It could be that you get messages like `no matches for kind "VirtualService" in version "networking.istio.io/v1beta1"`. This is normal if you install the operators in a cluster without a certain dependency. If you do not need this for your test, you can safely ignore it._

#### 3.3.3.2 Run on your local host

1. In https://github.com/kyma-project/lifecycle-manager run

   ```sh
   make run
   ```

2. In https://github.com/kyma-project/module-manager run

   ```sh
    make run
   ```

## 3.4 Start the Kyma installation

_Note for two cluster mode: Make sure you run the commands with `KUBECONFIG` set to the KCP Cluster_

1. First, install the module template in the control-plane to make it available for all Kyma installations:

   In `samples/template-operator`, run

   ```sh
   make module-template-push
   ```

   to apply the module-template.

2. Create a request for kyma installation of the module in `samples/template-operator` of the lifecycle-manager with

   ```sh
   sh hack/gen-kyma.sh
   
   # in single cluster setup, us this command:
   kubectl apply -f kyma.yaml singlecluster
   
   # in two cluster setup, use this command:
   kubectl apply -f kyma.yaml
   ```

3. Now try to check your kyma installation progress, e.g. with `kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P`:

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

### 3.4.1 Verify the installation

You can observe the installation in the runtime by switching the context to the SKR context and then verifying the status.

`kubectl get kymas.operator.kyma-project.io -n kyma-system -ojsonpath={".items[0].status"} | yq -P`

and it should show `state: Ready`.

You can verify this by checking if the contents of the `module-chart` directory in `template-operator/operator/module-chart` have been installed and parsed correctly.

You can even check the contents of the deployments that were generated by the deployed operator (assuming the helm chart did not change the name of the resource):
`kubectl get -f operator/module-chart/templates/deployment.yaml -ojsonpath={".status.conditions"} | yq`
