# Provision a Cluster and OCI Registry

## Context

This tutorial shows how to set up a cluster in different environments.

For the control-plane mode, with Kyma Control Plane (KCP) and Kyma runtime (SKR), create **two separate clusters** following the instructions below.

## Procedure

### Local Cluster Setup

1. Create a `k3d` cluster:

   ```sh
   k3d cluster create op-kcp --registry-create op-kcp-registry.localhost:8888
   
   # also add for the in-cluster mode only
   k3d cluster create op-skr --registry-create op-skr-registry.localhost:8888
   ```

2. Configure the local `k3d` registry. To reach the registries using `localhost`, add the following code to your `/etc/hosts` file:

   ```sh
   # Added for Operator Registries
   127.0.0.1 op-kcp-registry.localhost
   
   # also add for the in-cluster mode only
   127.0.0.1 op-skr-registry.localhost
   ```

3. Set the `IMG` environment variable for the `docker-build` and `docker-push` commands, to make sure images are accessible by local k3d clusters.

   * For the **single-cluster mode**:

      ```sh
      # pointing to KCP registry in dual cluster mode  
      export IMG=op-kcp-registry.localhost:8888/unsigned/operator-images
      ```

   * For the **control-plane mode**:

      ```sh
      # pointing to SKR registry in dual cluster mode
      export IMG=op-skr-registry.localhost:8888/unsigned/operator-images
      ```

4. Once you pushed your image, verify the content. For browsing through the content of the local container registry, use, for example, `http://op-kcp-registry.localhost:8888/v2/_catalog?n=100`.

### Remote Cluster Setup

Learn how to use a Gardener cluster for testing.

1. Go to the [Gardener account](https://dashboard.garden.canary.k8s.ondemand.com/account) and download your `Access Kubeconfig`.

2. Provision a compliant remote cluster:

   ```sh
   # name - name of the cluster
   # gardener_project - Gardener project name
   # gcp_secret - Cloud provider secret name (e.g. GCP)
   # gardener_account_kubeconfig - path to Access Kubeconfig from Step 1
   cat << EOF | kubectl apply --kubeconfig="${gardener_account_kubeconfig}" -f -
   apiVersion: core.gardener.cloud/v1beta1
   kind: Shoot
   metadata:
   name: ${name}
   spec:
   secretBindingName: ${gcp_secret}
   cloudProfileName: gcp
   region: europe-west3
   purpose: evaluation
   provider:
      type: gcp
      infrastructureConfig:
         apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
         kind: InfrastructureConfig
         networks:
         workers: 10.250.0.0/16
      controlPlaneConfig:
         apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
         kind: ControlPlaneConfig
         zone:  europe-west3-a
      workers:
      - name: cpu-worker
         minimum: 1
         maximum: 3
         machine:
         type: n1-standard-4
         volume:
         type: pd-standard
         size: 50Gi
         zones:
         - europe-west3-a
   networking:
      type: calico
      pods: 100.96.0.0/11
      nodes: 10.250.0.0/16
      services: 100.64.0.0/13
   hibernation:
      enabled: false
      schedules:
      - start: "00 14 * * ?"
         location: "Europe/Berlin"
   addons:
      nginxIngress:
         enabled: false
   EOF

   echo "waiting fo cluster to be ready..."
   kubectl wait --kubeconfig="${gardener_account_kubeconfig}" --for=condition=EveryNodeReady shoot/${name} --timeout=17m

   # create kubeconfig request, that creates a Kubeconfig, which is valid for one day
   kubectl create -kubeconfig="${gardener_account_kubeconfig}" \
      -f <(printf '{"spec":{"expirationSeconds":86400}}') \
      --raw /apis/core.gardener.cloud/v1beta1/namespaces/garden-${gardener_project}/shoots/${name}/adminkubeconfig | \
      jq -r ".status.kubeconfig" | \
      base64 -d > ${name}_kubeconfig.yaml

   # merge with the existing kubeconfig settings
   mkdir -p ~/.kube
   KUBECONFIG="~/.kube/config:${name}_kubeconfig.yaml" kubectl config view --merge > merged_kubeconfig.yaml
   mv merged_kubeconfig.yaml ~/.kube/config
   ```

3. Create an external registry.

   When using an external registry, make sure that the Gardener cluster (`op-kcpskr`) can reach your registry.

   You can follow the guide to [set up a GCP-hosted artifact registry (GCR)](prepare-gcr-registry.md).

   > **CAUTION:** For private registries, you may have to configure additional settings not covered in this tutorial.

4. Set the `IMG` environment variable for the `docker-build` and `docker-push` commands.

   ```sh
   # this an example
   # sap-kyma-jellyfish-dev is the GCP project
   # operator-test is the artifact registry
   export IMG=europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
   ```
