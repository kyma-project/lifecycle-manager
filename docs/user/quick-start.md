# Quick Start

This Quick Start guide will cover:

- Using kyma cli to provision control plane cluster and deploy lifecycle manager
- Deploy module template
- Using kyma cli manage modules

## Prerequisites
- [k3d](https://k3d.io/)
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/)
- [Kyma CLI](https://kyma-project.io/docs/kyma/latest/04-operation-guides/operations/01-install-kyma-CLI)

## Provision control plane cluster

Using following command to provision a local k3d cluster, by default, a cluster named `k3d-kyma` will be created, which play as control plane. 
```
kyma provision k3d
```
Since the services deployed in KCP are managed by istio, to have a similar local environment, it's also recommend to deploy istio into local k3d cluster.
```
istioctl install -y
```
Lifecycle manager also expose metrics which collected by prometheus operator in control plane to provide better observability, to simplify the local provision setup, you only need to deploy the ServiceMonitor CRD.
```
kubectl apply -f config/samples/tests/crds/servicemonitors.yaml
```
You can also follow official [quick start](https://prometheus-operator.dev/docs/prologue/quick-start/) guide to deploy prometheus operator into cluster as alternative solution if want to monitor component performance.

## Deploy lifecycle manager with
We recommend deploy lifecycle manager with control plane kustomize profile, and `kcp-system` and `kyma-system` namespace must be configured upfront.
```
kubectl create ns kcp-system
kubectl create ns kyma-system
kyma alpha deploy -c alpha -k https://github.com/kyma-project/lifecycle-manager/config/control-plane
```

If deploy works successfully, you should observe following main resources:

- Lifecycle-Manager named as `klm-controller-manager` deployment exits under `kcp-system` namespace
- All Module Templates configured in [kyma official repository](https://github.com/kyma-project/kyma/tree/main/modules) get deployed
- A Kyma CR use global `alpha` channel but without any module configured, sync disabled, named as `default-kyma` exists under `kyma-system` namespace
