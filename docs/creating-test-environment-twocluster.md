# Setup Guide for Two Clusters

We describe the setup of two clusters. One acts as control-plane (KCP) and another as Kyma runtime (SKR) equivalent.

**You can choose between a local setup or a external setup (using clusters managed by Gardener).**

## 1. Local setup

### 1.1 Create K3D clusters

```sh
k3d cluster create op-skr --registry-create op-skr-registry.localhost
k3d cluster create op-kcp --registry-create op-kcp-registry.localhost
```

### 1.2 Make sure `kubectl` is using the control-plane cluster

```sh
kubectl config use k3d-op-kcp
```

### 1.3 Configure local K3D registries

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
127.0.0.1 op-kcp-registry.localhost
127.0.0.1 op-skr-registry.localhost
```

#### 1.3.2 Set Registry Environment Variables

```sh
export MODULE_REGISTRY_PORT=$(docker port op-kcp-registry.localhost 5000/tcp | cut -d ":" -f2)
export IMG_REGISTRY_PORT=$(docker port op-skr-registry.localhost 5000/tcp | cut -d ":" -f2)

export IMG_REGISTRY=op-skr-registry.localhost:$IMG_REGISTRY_PORT/unsigned/operator-images
```

## 2. External setup

### 2.1 Create external clusters using Kyma CLI

Make sure to have two `KUBECONFIG` compliant client configurations at hand (one for control-plane and one for Kyma runtime).

Provision two compliant Kyma clusters with the `kyma-cli`:

```sh
kyma provision gardener gcp --name op-kcp --project jellyfish -s gcp-jellyfish-secret -c .kube/kubeconfig-garden-jellyfish.yaml
kyma provision gardener gcp --name op-skr --project jellyfish -s gcp-jellyfish-secret -c .kube/kubeconfig-garden-jellyfish.yaml
```

### 2.2 Create external registry

When using an external registry, make sure that both clusters (`op-kcp` and `op-skr`) can reach your registry.

_Disclaimer: For private registries, you may have to configure additional settings not covered in this tutorial. This only works out of the box for public registries_

You can follow this guide to [setup a GCP hosted artifact registry (GCR)](creating-test-environment-gcr.md).

#### 2.2.1 Set Registry Environment Variables

```sh
export MODULE_REGISTRY=your-registry-goes-here.com
export IMG_REGISTRY=your-registry-goes-here.com
```
