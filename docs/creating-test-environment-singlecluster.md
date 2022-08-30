# Setup Guide for Single Cluster

We describe how to setup a single cluster which acts as control-plane (KCP) and Kyma runtime (SKR) equivalent.

**You can choose between a local cluster setup or a remote cluster (based on Gardener) setup.**

## 1. Local setup

### 1.1 Create K3D cluster

```sh
k3d cluster create op-kcpskr --registry-create op-kcpskr-registry.localhost
```

### 1.2 Make sure `kubectl` is using the K3D cluster

```sh
kubectl config use k3d-op-kcpskr
```

### 1.3 Configure local K3D registry

#### 1.3.1 Make sure the registries are reachable via localhost

Add the following to your `/etc/hosts` file:

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

#### 1.3.2 Set registry environment variables

```sh
export MODULE_REGISTRY_PORT=$(docker port op-kcpskr-registry.localhost 5000/tcp | cut -d ":" -f2)
export IMG_REGISTRY_PORT=$(docker port op-kcpskr-registry.localhost 5000/tcp | cut -d ":" -f2)

export IMG_REGISTRY=op-kcpskr-registry.localhost:$IMG_REGISTRY_PORT/operator-images
```

#### 1.3.3 Web-UI for local container registry

For browsing through the content of the local container registry, run one of these tools
(both become accessible via http://localhost:8080):

* Crane Operator (http://localhost:8080)
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
* Docker Registry Browser (http://localhost:8080)
    ```sh
      docker run \
        -p 8080:8080 \
        --rm \
        --network=k3d-op-kcpskr \
        --name registry-browser \
        -e DOCKER_REGISTRY_URL=http://op-kcpskr-registry.localhost:5000 \
        klausmeyer/docker-registry-browser
     ```

## 2. External setup

This section describes how a Gardener cluster can be used for testing purposes.

### 2.1 Create external cluster using Kyma CLI

Provision a compliant Kyma Clusters with the [`kyma-cli`](https://github.com/kyma-project/cli):

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
