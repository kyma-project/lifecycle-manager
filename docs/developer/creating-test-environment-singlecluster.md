# Set up a test environment with a single cluster

In this document, you learn how to set up a test environment with a single cluster that acts as control plane (KCP) and Kyma runtime (SKR) equivalent; either locally or remotely (based on Gardener). For information about a test environment with two clusters, read [Set up a test environment with two clusters](creating-test-environment-twocluster.md).

## Local cluster setup

1. Create a K3D cluster:

   ```sh
   k3d cluster create op-kcpskr --registry-create op-kcpskr-registry.localhost:0.0.0.0:8888

2. Define that `kubectl` uses the K3D cluster:

   ```sh
   kubectl config use k3d-op-kcpskr

3. Configure the local K3D registry.

   3.1. To reach the registries using localhost, add the following code to your `/etc/hosts` file:

   ```
   # Added for Operator Registries
   127.0.0.1 op-kcpskr-registry.localhost
   ```

   3.2. Set the registry environment variables:

   ```sh
   export MODULE_REGISTRY=op-kcpskr-registry.localhost:8888/operator-demo               
   export IMG_REGISTRY=$MODULE_REGISTRY/operator-images

   ```

   3.3. View the content of your local container registry.

   For browsing through the content of the local container registry, `http://op-kcpskr-registry.localhost:8888/v2/_catalog?n=100`.


## Remote cluster setup

Learn how to use a Gardener cluster for testing.

1. Go to the the [Gardener account](https://dashboard.garden.canary.k8s.ondemand.com/account) and download your access credential called `${gardener_account_kubeconfig}`:.

2. Provision a compliant remote cluster with the [`kyma-cli`](https://github.com/kyma-project/cli):

   ```sh
   kyma provision gardener gcp --name op-kcpskr --project ${gardener_project} -s ${gcp_secret} -c ${gardener_account_kubeconfig}
   ```

   For example, this could look like `kyma provision gardener gcp --name op-kcpskr --project jellyfish -s gcp-jellyfish-secret -c .kube/kubeconfig-garden-jellyfish.yaml`

3. Create an external registry.

   When using an external registry, make sure that the Gardener cluster (`op-kcpskr`) can reach your registry.

   You can follow the guide to [set up a GCP-hosted artifact registry (GCR)](creating-test-environment-gcr.md).

   _Disclaimer: For private registries, you may have to configure additional settings not covered in this tutorial._


1. Set registry environment variables:

   ```sh
   export MODULE_REGISTRY=your-registry-goes-here.com
   export IMG_REGISTRY=your-registry-goes-here.com
