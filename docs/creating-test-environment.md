# Creating the Clusters

## Setup Control Plane and Runtime Equivalent

```sh
k3d cluster create op-skr --registry-create op-skr-registry.localhost
k3d cluster create op-kcp --registry-create op-kcp-registry.localhost
```

## Make sure the registries are reachable via localhost

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

## Make sure you are in the Control Plane

```
kubectl config use k3d-op-kcp
```

## Install CRDs of Operator Stack

### Install Manifest Operator CRDs

1. Checkout https://github.com/kyma-project/manifest-operator
2. Run the Installation Command

```
make install
```

### Install Kyma Operator CRDs

1. Checkout https://github.com/kyma-project/kyma-operator and `cd operator`
2. Run the Installation Command

```
make install
```

Ensure the CRDs are installed with `k get crds | grep kyma-project.io`:

```
manifests.component.kyma-project.io        2022-08-18T16:27:21Z
kymas.operator.kyma-project.io             2022-08-18T16:29:28Z
moduletemplates.operator.kyma-project.io   2022-08-18T16:29:28Z
```

## Build your module

In `https://github.com/kyma-project/kyma-operator`, `cd samples/template-operator`

After this find the Port of your KCP OCI Registry and write it to `MODULE_REGISTRY_PORT`:

```
export MODULE_REGISTRY_PORT=$(docker port op-kcp-registry.localhost 5000/tcp | cut -d ":" -f2)
echo $MODULE_REGISTRY_PORT
```

Next generate and push the module image of the operator

```
make module-operator-chart module-image
```

First make sure that the `kyma-system` namespace is created:

```
kubectl create ns kyma-system
```

## Use your module and trigger a Kyma Installation

After this, build and push the module the module with

```
make module-build module-template-push
```

Before we start reconciling, lets create a secret to access the SKR:

In https://github.com/kyma-project/kyma-operator run

`sh operator/config/samples/secret/k3d-secret-gen.sh`

## Run the operators

In https://github.com/kyma-project/kyma-operator run

```
make run
```

In https://github.com/kyma-project/manifest-operator run

```
make run
```

## Start the Installation

Create a request for kyma installation of the module with

```
sh hack/gen-kyma.sh
kubectl apply -f kyma.yaml
```

Now try to check your kyma installation progress, e.g. with `k get kyma -n kyma-system -ojsonpath={".items[0].status"} | yq -P`:

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
