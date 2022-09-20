# Set up a test environment with a single cluster

In this document, you learn how to set up a test environment with a single cluster that acts as control plane (KCP) and Kyma runtime (SKR) equivalent; either locally or remotely (based on Gardener). 
For information about a test environment with two clusters, read [Set up a test environment with two clusters](creating-test-environment-twocluster).


## Local cluster setup

1. Create a K3D cluster:

   ```sh
   k3d cluster create op-kcpskr --registry-create op-kcpskr-registry.localhost

2. Define that `kubectl` uses the K3D cluster:

   ```sh
   kubectl config use k3d-op-kcpskr

3. Configure the local K3D registry.

   3.1. To reach the registries using localhost, add the following code to your `/etc/hosts` file:


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
127.0.0.1 op-kcpskr-registry.localhost op-kcp-registry.localhost op-skr-registry.localhost
```

   3.2. Set the registry environment variables:

      ```sh
export MODULE_REGISTRY_PORT=$(docker port op-kcpskr-registry.localhost 5000/tcp | cut -d ":" -f2)
export IMG_REGISTRY_PORT=$(docker port op-kcpskr-registry.localhost 5000/tcp | cut -d ":" -f2)

export IMG_REGISTRY=op-kcpskr-registry.localhost:$IMG_REGISTRY_PORT/unsigned/operator-images

export KCP_CLUSTER_CTX=k3d-op-kcpskr
export SKR_CLUSTER_CTX=k3d-op-kcpskr
```

   3.3. View the content of your local container registry.

      For browsing through the content of the local container registry, run one of the following tools. Both are accessible with `http://localhost:8080`.

      - Crane Operator (http://localhost:8080)
        ```sh
    docker run \
     -p 8080:80 \
     --rm \
     --network=k3d-op-kcpskr \
     --name=docker_registry_ui \
     -e REGISTRY_HOST=op-kcpskr-registry.localhost \
     -e REGISTRY_PORT=5000 \
     -e REGISTRY_PROTOCOL=http \
     -e ALLOW_REGISTRY_LOGIN=false \
     -e REGISTRY_ALLOW_DELETE=true \
     parabuzzle/craneoperator:latest
  ```
      - Docker Registry Browser (http://localhost:8080)
        ```sh
    docker run \
      -p 8080:8080 \
      --rm \
      --network=k3d-op-kcpskr \
      --name registry-browser \
      -e DOCKER_REGISTRY_URL=http://op-kcpskr-registry.localhost:5000 \
      klausmeyer/docker-registry-browser
  ```

## Remote cluster setup

Learn how to use a Gardener cluster for testing.

1. Go to the the [Gardener account](https://dashboard.garden.canary.k8s.ondemand.com/account) and download your access credential called `${gardener_account_kubeconfig}`:.



```sh
kyma provision gardener gcp --name op-kcpskr --project ${gardener_project} -s ${gcp_secret} -c ${gardener_account_kubeconfig}
```

this is how it could like:

```sh
kyma provision gardener gcp --name op-kcpskr --project jellyfish -s gcp-jellyfish-secret -c .kube/kubeconfig-garden-jellyfish.yaml
```

### 2.2 Create external registry

When using an external registry, make sure that the Gardener cluster (`op-kcpskr`) can reach your registry.

_Disclaimer: For private registries, you may have to configure additional settings not covered in this tutorial. This only works out of the box for public registries_

You can follow this guide to [setup a GCP hosted artifact registry (GCR)](creating-test-environment-gcr.md).

#### 2.2.1 Set registry environment variables

```sh
export MODULE_REGISTRY=your-registry-goes-here.com
export IMG_REGISTRY=your-registry-goes-here.com
```
