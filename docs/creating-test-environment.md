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



5. To install the Module Manager CRDs, check out `https://github.com/kyma-project/module-manager`, navigate to the operator `cd operator`, and run `make install`.




6. To install the Lifecycle Manager CRDs, check out `https://github.com/kyma-project/lifecycle-manager`, navigate to the operator `cd operator`, and run `make install`.




7. To verify that the CRDs are installed, run `kubectl get crds | grep kyma-project.io`.
   You should get a result similar to this:

   manifests.component.kyma-project.io        2022-08-18T16:27:21Z
   kymas.operator.kyma-project.io             2022-08-18T16:29:28Z
   moduletemplates.operator.kyma-project.io   2022-08-18T16:29:28Z

8. Deploy and run the operators in the cluster. Due to cross-dependencies in CRDs, install them in the following order:
   1. In `https://github.com/kyma-project/module-manager/operator run`, run:

#### 3.3.3.1 Deploy and run operators in cluster



      ```sh
      # using local registry
      make docker-build docker-push deploy IMG=$IMG_REGISTRY/module-manager:dev
      
      # using remote registry (replace `PR-73` with your desired tag)
      make deploy IMG=eu.gcr.io/kyma-project/module-manager:PR-73
```

   2. In `https://github.com/kyma-project/lifecycle-manager/operator`, run:
      
      ```sh
      # using local registry
      make docker-build docker-push deploy IMG=$IMG_REGISTRY/lifecycle-manager:dev
      
      # using remote registry (replace `PR-122` with your desired tag)
      make deploy IMG=eu.gcr.io/kyma-project/lifecycle-manager:PR-122
      ```

   > **NOTE:** If you get messages like `no matches for kind "VirtualService" in version "networking.istio.io/v1beta1"`, don't worry. This is normal if you install the operators in a cluster without a certain dependency. If you do not need this dependency for your test, you can safely ignore the warning.

9. Run the operators on your local host. Due to cross-dependencies in CRDs, install them in the following order:
   1. In `https://github.com/kyma-project/lifecycle-manager`, run `make run`.
   2. In `https://github.com/kyma-project/module-manager`, run `make run`.





10. Start the Kyma installation.


_Note for using Google Artifact Registry: if you use gcloud cli, make sure your local docker config (`~/.docker/config.json`) does not contains `gcloud` `credHelpers` entry, (e.g: `"europe-west3-docker.pkg.dev": "gcloud",`), otherwise this might cause authentication issue for module-manager while fetching remote oci image layer.
    1. To make the module template available for all Kyma installations, install it in the control plane.

      To apply the module template, go to `samples/template-operator` and run `make module-template-push`.



   2. To create a request for Kyma installation of the module in `samples/template-operator` of the Lifecycle Manager, run:

      ```sh
      sh hack/gen-kyma.sh
   
      # in single cluster setup, us this command:
      kubectl apply -f kyma.yaml singlecluster
   
      # in two-cluster setup, use this command:
      kubectl apply -f kyma.yaml
      ```

   3. Check the progress of your Kyma installation; for example, with `kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P`.
      You should get a result like this:

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
