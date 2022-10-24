# Provision cluster and OCI registry

In this document, you will learn how to set up a cluster in different environments.

For a dual cluster setup, with control plane (KCP) and Kyma runtime (SKR); create **two separate clusters** following the instructions below.

## Local cluster setup

1. Create a `k3d` cluster:

   ```sh
   k3d cluster create op-kcp --registry-create op-kcp-registry.localhost:0.0.0.0:8888
   
   # also add for dual cluster mode only
   k3d cluster create op-skr --registry-create op-skr-registry.localhost:0.0.0.0:8888
   ```
2. Configure the local `k3d` registry. 
   1. To reach the registries using `localhost`, add the following code to your `/etc/hosts` file:

      ```sh
      # Added for Operator Registries
      127.0.0.1 op-kcp-registry.localhost
   
      # also add for dual cluster mode only
      127.0.0.1 op-skr-registry.localhost
      ```

   2. Use the module operator registry:

      ```sh
      export MODULE_REGISTRY=op-kcp-registry.localhost:8888/unsigned 
      ```
   3. Use the module image registry:
      
      In **_single cluster mode_**:
      ```sh
      # pointing to SKR registry in dual cluster mode  
      export IMG_REGISTRY=op-kcp-registry.localhost:8888/unsigned/operator-images
      ```
      In **_dual cluster mode_**:
      ```sh
      # pointing to SKR registry in dual cluster mode
      export IMG_REGISTRY=op-skr-registry.localhost:8888/unsigned/operator-images
      ```

3. View the content of your local container registry. For browsing through the content of the local container registry, e.g. `http://op-kcp-registry.localhost:8888/v2/_catalog?n=100`.


## Remote cluster setup

Learn how to use a Gardener cluster for testing.

1. Go to the [Gardener account](https://dashboard.garden.canary.k8s.ondemand.com/account) and download your `Access Kubeconfig`.

2. Provision a compliant remote cluster with the [`kyma-cli`](https://github.com/kyma-project/cli):

   ```sh
   # gardener_project - Gardener project name
   # gcp_secret - Cloud provider secret name (e.g. GCP)
   # gardener_account_kubeconfig - path to Access Kubeconfig from Step 1
   kyma provision gardener gcp --name op-kcpskr --project ${gardener_project} -s ${gcp_secret} -c ${gardener_account_kubeconfig}
   ```

   For example, this could look like `kyma provision gardener gcp --name op-kcpskr --project jellyfish -s gcp-jellyfish-secret -c .kube/kubeconfig-garden-jellyfish.yaml`

3. Create an external registry

   When using an external registry, make sure that the Gardener cluster (`op-kcpskr`) can reach your registry.

   You can follow the guide to [set up a GCP-hosted artifact registry (GCR)](prepare-gcr-registry.md).

   _Disclaimer: For private registries, you may have to configure additional settings not covered in this tutorial._