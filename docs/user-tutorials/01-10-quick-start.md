# Control Plane Quick Start

This quick start guide shows how to:

- provision a Kyma Control Plane (KCP) cluster and deploy Lifecycle Manager using Kyma CLI
- deploy a ModuleTemplate CR
- manage modules using Kyma CLI

## Prerequisites

To use Lifecycle Manager in a local setup, you need the following prerequisites:

- [k3d](https://k3d.io/)
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/)
- [Kyma CLI](https://kyma-project.io/docs/kyma/latest/04-operation-guides/operations/01-install-kyma-CLI)

## Provision a KCP cluster

1. Provision a local k3d cluster as KCP. By default, a cluster named `k3d-kyma` is created. Run:

   ```bash
   kyma provision k3d
   ```

2. Because the services deployed in KCP are managed by Istio, you need to install Istio into the local k3d cluster. Run:

   ```bash
   istioctl install -y
   ```

3. Lifecycle Manager exposes metrics that are collected by Prometheus Operator in KCP to provide better observability. To simplify the local setup, you only need to deploy the ServiceMonitor CRD using the following command:

   ```bash
   kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/main/charts/kube-prometheus-stack/crds/crd-servicemonitors.yaml
   ```

You can also follow the official [Prometheus Operator quick start](https://prometheus-operator.dev/docs/prologue/quick-start/) guide to deploy a full set of Prometheus Operator features into your cluster.

## Deploy Lifecycle Manager

We recommend deploying Lifecycle Manager with the KCP kustomize profile. You must create `kcp-system` and `kyma-system` Namespaces before the actual deployment. Run:

   ```bash
   kubectl create ns kcp-system
   kubectl create ns kyma-system
   kyma alpha deploy -k https://github.com/kyma-project/lifecycle-manager/config/control-plane
   ```

If the deployment was successful, you should see all the required resources. For example:

- The `klm-controller-manager` Pod in the `kcp-system` Namespace
- A Kyma CR that uses the `regular` channel but without any module configured, sync disabled, named `default-kyma` under `kyma-system` Namespace

### Manage modules in remote cluster mode

To allow Kyma Lifecycle Manager manages Kyma modules in remote cluster, two prerequisite must be fulfilled.

1. The Lifecycle Manager must deploy with control-plane kustomize profile.
2. A kubernetes secret resource which contains remote cluster kubeconfig access data must be deployed into control plane cluster upfront.

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

- For publishing Module Templates in a private OCI registry, refer to our [Private Registry Configuration Guide](tutorials/config-private-registry.md)
- For managing module initialization with the provided strategies, refer to our [Managing module initialization with the CustomResourcePolicy](tutorials/manage-module-with-custom-resource-policy.md)