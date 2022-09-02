# 1. Creating the cluster

You can choose between a single cluster setup or a two cluster setup for testing your Kyma module.

## 1.1. Single cluster with registry

For using a single cluster which acts as control-plane (KCP) and Kyma runtime (SKR) together, follow the [single cluster setup guide](creating-test-environment-singlecluster.md).

## 1.2. Two clusters with two Registries

For testing with two clusters, one representing the KCP and another the SKR cluster, follow the [two cluster setup guide](creating-test-environment-twocluster.md).

# 2. Build your module

In this example we use our reference implementation for a reconciliation operator
(see the [`template-operator` in Github](https://github.com/kyma-project/lifecycle-manager/tree/main/samples/template-operator))
to demonstrate how the bundling to a Kyma module works.

1. Switch to Your Operator Folder

   In `https://github.com/kyma-project/lifecycle-manager`, `cd samples/template-operator`

2. Generating and Pushing the Operator Image and Charts

   Next generate and push the module image of the operator.

   ```sh
   make module-operator-chart module-image
   ```

# 3. Install Kyma with your module

## 3.1 Pre-requisites

_Note for two cluster mode: set your `KUBECONFIG` to the KCP Cluster context_

First make sure that the `kyma-system` namespace is created:

```sh
kubectl create ns kyma-system
```

Create a secret to access the cluster which acts as SKR:

In https://github.com/kyma-project/lifecycle-manager, run these commands

```
export KCP_CLUSTER_CTX=k3d-op-kcpskr
export SKR_CLUSTER_CTX=k3d-op-kcpskr
chmod 755 ./operator/config/samples/secret/k3d-secret-gen.sh
./operator/config/samples/secret/k3d-secret-gen.sh
```

_Note for two cluster mode: You can use KCP_CLUSTER_CTX and SKR_CLUSTER_CTX to adjust your contexts for applying the secret._

## 3.2 Build and push the module

In https://github.com/kyma-project/lifecycle-manager, switch to subfolder `samples/template-operator` and run this command to build the module and push it to the registry:

```sh
make module-build
```

_Note: If you use a remote registry and you receive 403 / 401, recreate the `MODULE_CREDENTIALS` variable as it could be that your credentials timed out_

## 3.3 Run the operators

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

In https://github.com/kyma-project/module-manager run

```sh
# using local registry
make docker-build docker-push deploy IMG=$IMG_REGISTRY/module-manager:dev

# using remote registry (replace `PR-73` with your desired tag)
make deploy IMG=eu.gcr.io/kyma-project/module-manager:PR-73
```

In https://github.com/kyma-project/lifecycle-manager run

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
