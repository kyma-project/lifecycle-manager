# Quick Start

This Quick Start guide will cover:

- Using kyma cli to provision control plane cluster and deploy Kyma Lifecycle Manager
- Deploy module template
- Using kyma cli manage modules

## Prerequisites
To use Kyma Lifecycle Manager for local setup, you need to have the following prerequisites:

- [k3d](https://k3d.io/)
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/)
- [Kyma CLI](https://kyma-project.io/docs/kyma/latest/04-operation-guides/operations/01-install-kyma-CLI)

## Provisioning a Control Plane Cluster with required resources

You can provision a local k3d cluster as a control plane using the following command, by default, a cluster named `k3d-kyma` will be created. 
```
kyma provision k3d
```
Since the services deployed in the control plane are managed by Istio, you need to install Istio into the local k3d cluster using the following command:
```
istioctl install -y
```
Lifecycle Manager also exposes metrics that are collected by Prometheus Operator in the control plane to provide better observability. To simplify the local setup, you only need to deploy the ServiceMonitor CRD using the following command:
```
kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/main/charts/kube-prometheus-stack/crds/crd-servicemonitors.yaml
```
You can also follow official [quick start](https://prometheus-operator.dev/docs/prologue/quick-start/) guide to deploy a full set of prometheus operator into cluster as an alternative solution if you want to monitor the component performance.

## Deploy Kyma lifecycle manager
We recommend deploying Kyma Lifecycle Manager with the control plane kustomize profile, and `kcp-system`, `kyma-system` namespace must be configured upfront.
```
kubectl create ns kcp-system
kubectl create ns kyma-system
kyma alpha deploy -c alpha -k https://github.com/kyma-project/lifecycle-manager/config/control-plane
```

If the deployment is successful, you should see the following main resources:

- Kyma Lifecycle Manager deployment instance named as `klm-controller-manager` under `kcp-system` namespace
- All Module Templates configured in [kyma official repository](https://github.com/kyma-project/kyma/tree/main/modules) get deployed in `control-plane` as target
- A Kyma CR that uses the global `alpha` channel but without any module configured, sync disabled, named `default-kyma` under `kyma-system` namespace

### Manage modules in single cluster mode
By default, all Module Templates deployed by Kyma CLI will be configured for single cluster mode (`spec.target=control-plane`), which means the module will be deployed in the Control Plane cluster, together with Kyma Lifecycle Manager.

With Kyma CLI [enable module](https://github.com/kyma-project/cli/blob/main/docs/gen-docs/kyma_alpha_enable.md) and [disable module](https://github.com/kyma-project/cli/blob/main/docs/gen-docs/kyma_alpha_disable.md) command, you can manage kyma modules without directly modify Kyma CR.

As an example, you can use following command to enable keda module in `alpha` channel for `default-kyma` under `kyma-system` namespace
```
kyma alpha enable module keda -c alpha -n kyma-system -k default-kyma
```
### Manage modules in remote cluster mode
To allow Kyma Lifecycle Manager manages Kyma modules in remote cluster, two prerequisite must be fulfilled.

1. For the module to be deployed remotely, the related Module Template must be configured as `remote` in `spec.target`.
2. A kubernetes secret resource which contains remote cluster kubeconfig access data must be deployed into control plane cluster upfront.

The Module Template persists in [kyma official repository](https://github.com/kyma-project/kyma/tree/main/modules) are configured as `remote` by default, you can use following command deploy them:
```
kubectl apply -k https://github.com/kyma-project/kyma//modules
```

In order to manage remote cluster modules, Kyma Lifecycle Manager needs to know the authentication credential, like other native Kubernetes tools, the nature way to communicate with Kubernetes API server is through kubeconfig file. 

That brings us the design idea to relay on the secret resource to provide this information. In each secret, there configured a label named `operator.kyma-project.io/kyma-name`, user must configure the label value same as the Kyma CR name so that Lifecyle Manager can knows which correct authentication credential to use.

With the following command, it will create a secret yaml file which named `default-kyma` (same as the Kyma CR name) under `kyma-system` (same as the Kyma CR namespace) namespace, which contains remote cluster kubeconfig as `data.config`.
```
export KUBECONFIG=[path to your remote cluster kubeconfig yaml file]
./hack/k3d-secret-gen.sh default-kyma kyma-system
```
Deploy this secret under local control plane cluster:
```
kubectl config use-context k3d-kyma 
kubectl apply -f default-kyma-secret.yaml
```

After this access secrete successfully deployed, you can start to use Kyma CLI manage modules for remote cluster.

# Next Steps

- For publish Module Template in private OCI registry refer to our [Provide credential for private OCI registry authentication](tutorials/config-private-registry.md)