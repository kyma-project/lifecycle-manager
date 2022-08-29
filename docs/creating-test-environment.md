# Creating the Clusters

## Setup Control Plane and Runtime Equivalent

### Local Setup

```sh
k3d cluster create op-skr --registry-create op-skr-registry.localhost
k3d cluster create op-kcp --registry-create op-kcp-registry.localhost
```

### Use External Clusters (through Kyma CLI)

Make sure to have two `KUBECONFIG` compliant client configurations at hand, one for kcp, one for skr.

Provision two compliant kyma Clusters with the `kyma-cli`:

```sh
kyma provision gardener gcp --name op-kcp --project jellyfish -s gcp-jellyfish-secret -c .kube/kubeconfig-garden-jellyfish.yaml
kyma provision gardener gcp --name op-skr --project jellyfish -s gcp-jellyfish-secret -c .kube/kubeconfig-garden-jellyfish.yaml
```

## Setting up your registry

### Make sure the registries are reachable via localhost (only for local setup)

Add the following to your `etc/hosts` entry.

```/etc/hosts
##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1       localhost
255.255.255.255 broadcasthost
::1             localhost

# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal

# Added for Operator Registries
127.0.0.1 op-kcp-registry.localhost
127.0.0.1 op-skr-registry.localhost
```

### Using an external Registry

When using an external registry, make sure that both clusters (`op-kcp` and `op-skr`) can reach your registry.

_Disclaimer: For private registries, you may have to configure additional settings not covered in this tutorial. This only works out of the box
for public registries_

## Make sure you are in the Control Plane

```sh
kubectl config use k3d-op-kcp
```

# Build your module

In `https://github.com/kyma-project/lifecycle-manager`, `cd samples/template-operator`

After this find the Port of your KCP OCI Registry and write it to `MODULE_REGISTRY_PORT`:

## Using a Local Module/Image Registry

```sh
export MODULE_REGISTRY_PORT=$(docker port op-kcp-registry.localhost 5000/tcp | cut -d ":" -f2)
export IMG_REGISTRY_PORT=$(docker port op-skr-registry.localhost 5000/tcp | cut -d ":" -f2)
```

## Using a Remote Module/Image Registry

In general its possible to update your registries with 2 environment variables (for the module template, and the operator image):

```sh
export MODULE_REGISTRY=your-registry-goes-here.com
export IMG_REGISTRY=your-registry-goes-here.com
```

### Using GCP Artifact Registry

We will be assuming you have a GCP project called `sap-kyma-jellyfish-dev`

#### Creating your Repository

We will assume you will be creating and using a Artifact Registry Repository called `operator-test`.

```sh
gcloud artifacts repositories create operator-test \
    --repository-format=docker \
    --location europe-west3
```

```sh
export MODULE_REGISTRY=europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
export IMG_REGISTRY=$MODULE_REGISTRY/operator-images
```

_Note: For `MODULE_REGISTRY` it is important not to define any scheme such as `https://` so that the module generation works correctly, it is appended automatically in the operators based on the environment_

Now, make sure that the Read access to the repository is possible anonymously to make it work with remote clusters (e.g. in gardener)

```sh
gcloud artifacts repositories add-iam-policy-binding operator-test \
 --location=europe-west3 --member=allUsers --role=roles/artifactregistry.reader
```

#### Authenticating Locally

We will assume you will be creating and using a service-account called `operator-test-sa`.

Make sure to authenticate against your registry:

```sh
gcloud auth configure-docker \
    europe-west3-docker.pkg.dev
```

#### Creating a service Account

Creation of a service account is useful for productive purposes

Create a Service Account (for the necessary permissions see https://cloud.google.com/iam/docs/creating-managing-service-accounts#permissions):

```sh
gcloud iam service-accounts create operator-test-sa \
    --display-name="Operator Test Service Account"
```

```sh
gcloud projects add-iam-policy-binding sap-kyma-jellyfish-dev \
      --member='serviceAccount:operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com' \
      --role='roles/artifactregistry.reader' \
      --role='roles/artifactregistry.writer'
```

Impersonate the service-account

```sh
gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com
```

Verify your login:

```sh
gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com | docker login -u oauth2accesstoken --password-stdin https://europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
```

```sh
export MODULE_CREDENTIALS=oauth2accesstoken:$(gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com)
```

## Generating and Pushing the Operator Image and Charts

Next generate and push the module image of the operator

```sh
make module-operator-chart module-image
```

# Install Kyma with your Module

_Note for Remote Clusters: Make sure you run the commands with `KUBECONFIG` set to the KCP Cluster_

First make sure that the `kyma-system` namespace is created:

```sh
kubectl create ns kyma-system
```

After this, build the module and push it to the registry:

```sh
make module-build
```

_Note: If you receive 403 / 401, recreate the `MODULE_CREDENTIALS` variable as it could be that your credentials timed out_

Before we start reconciling, let's create a secret to access the SKR:

In https://github.com/kyma-project/lifecycle-manager in the `operator` subdirectory, run

`sh config/samples/secret/k3d-secret-gen.sh`

_Note for externally created clusters: You can use KCP_CLUSTER_CTX and SKR_CLUSTER_CTX to adjust your contexts for applying the secret._

## Run the operators

### Run Locally

_Note for Remote Clusters: Make sure you run the commands with `KUBECONFIG` set to the KCP Cluster_

#### Install Manifest Operator CRDs

1. Checkout https://github.com/kyma-project/module-manager and navigate to the operator: `cd operator`
2. Run the Installation Command

```sh
make install
```

#### Install lifecycle-manager CRDs

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

#### Run the Operators

_Note for Remote Clusters: Make sure you run the commands with `KUBECONFIG` set to the KCP Cluster_

In https://github.com/kyma-project/lifecycle-manager run

```sh
make run
```

In https://github.com/kyma-project/module-manager run

```sh
make run
```

### Run in Control Plane

_Note: The order of installation is important due to cross-dependencies in CRDs_

In https://github.com/kyma-project/module-manager run

```sh
make deploy IMG=eu.gcr.io/kyma-project/module-manager:PR-73
```

_Note: Replace `PR-73` with your desired tag_

In https://github.com/kyma-project/lifecycle-manager run

```sh
make deploy IMG=eu.gcr.io/kyma-project/lifecycle-manager:PR-122
```

_Note: Replace `PR-122` with your desired tag_

_Note: It could be that you get messages like `no matches for kind "VirtualService" in version "networking.istio.io/v1beta1"`. This is normal if you install the operators in a cluster without a certain dependency. If you do not need this for your test, you can safely ignore it._

## Start the Installation

_Note for Remote Clusters: Make sure you run the commands with `KUBECONFIG` set to the KCP Cluster_

First, install the module template in the control-plane to make it available for all Kyma installations:

In `samples/template-operator`, run

```sh
make module-template-push
```

to apply the module-template

Create a request for kyma installation of the module in `samples/template-operator` of the lifecycle-manager with

```sh
sh hack/gen-kyma.sh
kubectl apply -f kyma.yaml
```

Now try to check your kyma installation progress, e.g. with `kubectl get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P`:

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
        group: component.kyma-project.io
        kind: Manifest
        version: v1alpha1
observedGeneration: 1
state: Ready
```

Also, you can observe the installation in the runtime by switching the context to the SKR context and then verifying the status.

`kubectl config use-context k3d-op-skr && kubectl get samples.component.kyma-project.io -n kyma-system -ojsonpath={".items[0].status"} | yq -P`

and it should show `state: Ready`.

You can verify this by checking if the contents of the `module-chart`directry in `template-operator/operator/module-chart`have been installed and parsed correctly.

You can even check the contents of the deployments that were generated by the deployed operator (assuming the helm chart did not change the name of the resource):
`kubectl get -f operator/module-chart/templates/deployment.yaml -ojsonpath={".status.conditions"} | yq`
