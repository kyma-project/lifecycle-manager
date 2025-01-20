# Local Test Setup in the Control Plane Mode Using k3d

> ### Supported Versions
> For more information on the tooling versions expected in the project, see [`versions.yaml`](../../versions.yaml).

## Context

This tutorial shows how to configure a fully working e2e test setup including the following components:

* Lifecycle Manager
* Runtime Watcher on a remote cluster
* `template-operator` on a remote cluster as an example

This setup is deployed with the following security features enabled:

* Strict mTLS connection between Kyma Control Plane (KCP) and SKR clusters
* SAN Pinning (SAN of client TLS certificate needs to match the DNS annotation of a corresponding Kyma CR)

## Prerequisites

The following tooling is required in the versions defined in [`versions.yaml`](../../versions.yaml):

- docker
- go
- golangci-lint
- istioctl
- k3d
- kubectl
- kustomize
- [modulectl](https://github.com/kyma-project/modulectl)
- yq

## Procedure

Execute the following scripts from the project root.

## Create Test Clusters

Create local test clusters for SKR and KCP.

```sh
K8S_VERSION=$(yq e '.k8s' ./versions.yaml)
CERT_MANAGER_VERSION=$(yq e '.certManager' ./versions.yaml)
./scripts/tests/create_test_clusters.sh --k8s-version $K8S_VERSION --cert-manager-version $CERT_MANAGER_VERSION
```

## Install the CRDs

Install the CRDs to the KCP cluster.

```sh
./scripts/tests/install_crds.sh
```

## Deploy lifecycle-manager

Deploy a built image from the registry, e.g. the `latest` image from the `prod` registry.

```sh
REGISTRY=prod
TAG=latest
./scripts/tests/deploy_klm_from_registry.sh --image-registry $REGISTRY --image-tag $TAG
```

OR build a new image from the local sources, push it to the local KCP registry and deploy it.

```sh
./scripts/tests/deploy_klm_from_sources.sh
```

## Deploy a Kyma CR

```sh
SKR_HOST=host.k3d.internal
./scripts/tests/deploy_kyma.sh $SKR_HOST
```

## Verify if the Kyma becomes Ready

Verify Kyma is Ready in KCP (takes roughly 1-2 minutes).

```sh
kubectl config use-context k3d-kcp
kubectl get kyma/kyma-sample -n kcp-system
```

Verify Kyma is Ready in SKR (takes roughly 1-2 minutes).

```sh
kubectl config use-context k3d-skr
kubectl get kyma/default -n kyma-system
```

## [OPTIONAL] Deploy template-operator module

Build it locally and deploy it.

```sh
cd <template-operator-repository>

make build-manifests
modulectl create --config-file ./module-config.yaml --registry http://localhost:5111 --insecure 

kubectl config use-context k3d-kcp
# repository URL is localhost:5111 on the host machine but must be k3d-kcp-registry.localhost:5000 within the cluster
yq e '.spec.descriptor.component.repositoryContexts[0].baseUrl = "k3d-kcp-registry.localhost:5000"' ./template.yaml | kubectl apply -f -

MT_VERSION=$(yq e '.spec.version' ./template.yaml)
cd <lifecycle-manager-repository>
./scripts/tests/deploy_modulereleasemeta.sh template-operator regular:$MT_VERSION
```

## [OPTIONAL] Add the template-operator module to the Kyma CR and verify if it becomes Ready

Add the module.

```sh
kubectl config use-context k3d-skr
kubectl get kyma/default -n kyma-system -o yaml | yq e '.spec.modules[0]={"name": "template-operator"}' | kubectl apply -f -
```

Verify if the module becomes ready (takes roughly 1-2 minutes).

```sh
kubectl config use-context k3d-skr
kubectl get kyma/default -n kyma-system -o wide
```

To remove the module again.

```sh
kubectl config use-context k3d-skr
kubectl get kyma/default -n kyma-system -o yaml | yq e 'del(.spec.modules[0])' | kubectl apply -f -
```

## [OPTIONAL] Verify conditions

Check the conditions of the Kyma.

- `SKRWebhook` to determine if the webhook has been installed to the SKR
- `ModuleCatalog` to determine if the ModuleTemplates and ModuleReleaseMetas haven been synced to the SKR cluster
- `Modules` to determine if the added modules are ready

```sh
kubectl config use-context k3d-kcp
kubectl get kyma/kyma-sample -n kcp-system -o yaml | yq e '.status.conditions'
```

## [OPTIONAL] Verify if watcher events reach KCP

Flick the channel to trigger an event.

```sh
kubectl config use-context k3d-skr
kubectl get kyma/default -n kyma-system -o yaml | yq e '.spec.channel="regular"' | kubectl apply -f -
kubectl get kyma/default -n kyma-system -o yaml | yq e '.spec.channel="fast"' | kubectl apply -f -
```

 Verify if lifecyle-manger received the event on KCP.

```sh
kubectl config use-context k3d-kcp
kubectl logs deploy/klm-controller-manager -n kcp-system | grep "event received from SKR"
```

## [OPTIONAL] Delete the local test clusters

Remove the local SKR and KCP test clusters.

```shell
k3d cluster rm kcp skr
```
