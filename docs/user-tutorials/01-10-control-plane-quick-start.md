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

2. Because the services deployed in KCP are managed by Istio, you need to install Istio on the local k3d cluster. Run:

   ```bash
   istioctl install -y
   ```

3. Lifecycle Manager exposes metrics that are collected by Prometheus Operator in KCP to provide better observability. To simplify the local setup, you only need to deploy the ServiceMonitor CRD using the following command:

   ```bash
   kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/main/charts/kube-prometheus-stack/crds/crd-servicemonitors.yaml
   ```

You can also follow the official [Prometheus Operator quick start](https://prometheus-operator.dev/docs/prologue/quick-start/) guide to deploy a full set of Prometheus Operator features into your cluster.

## Deploy Lifecycle Manager

We recommend deploying Lifecycle Manager with the KCP kustomize profile. You must create the `kcp-system` and `kyma-system` Namespaces before the actual deployment. Run:

   ```bash
   kubectl create ns kcp-system
   kubectl create ns kyma-system
   kyma alpha deploy -k https://github.com/kyma-project/lifecycle-manager/config/control-plane
   ```

If the deployment was successful, you should see all the required resources. For example:

- The `klm-controller-manager` Pod in the `kcp-system` Namespace
- A Kyma CR that uses the `regular` channel but without any module configured, sync disabled, named `default-kyma` under `kyma-system` Namespace

### Manage modules in the control-plane mode

To manage Kyma modules in the control-plane mode, Lifecycle Manager requires:

1. Deployment with the control-plane kustomize profile.
2. A Kubernetes Secret resource that contains remote cluster kubeconfig access data deployed on KCP cluster.

In order to manage remote cluster modules, Lifecycle Manager needs to know the authentication credentials. Just like with any other native Kubernetes tool, the natural way to communicate with Kubernetes API server is through a kubeconfig file.

That brings us the design idea to rely on the Secret resource to provide the credentials. Each Secret, has the `operator.kyma-project.io/kyma-name` label. The user must configure the label values with the same name and Namespace as the Kyma CR so that Lifecycle Manager can knows which authentication credentials to use.

1. Create a Secret yaml file named `default-kyma` (the same as the Kyma CR name) in the `kyma-system` Namespace (the same as the Kyma CR Namespace), which contains the remote cluster kubeconfig as `data.config`. Run:

   ```bash
   export KUBECONFIG=[path to your remote cluster kubeconfig yaml file]
   ./hack/k3d-secret-gen.sh default-kyma kyma-system
   ```

2. Deploy the Secret on the local KCP cluster:

   ```bash
   kubectl config use-context k3d-kyma 
   kubectl apply -f default-kyma-secret.yaml
   ```

After the successful deployment of the access Secrete, you can start to use Kyma CLI to manage modules on remote clusters.

## Next Steps

- To learn how to publish ModuleTemplate CRs in a private OCI registry, refer to the [Provide credentials for private OCI registry authentication](../developer-tutorials/config-private-registry.md) tutorial
- To learn how to manage module enablement with the provided strategies, refer to the [Manage module enablement with the CustomResourcePolicy](02-10-manage-module-with-custom-resource-policy.md/) tutorial
