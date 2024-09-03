<!-- markdown-link-check-disable-next-line -->
[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/lifecycle-manager)](https://api.reuse.software/info/github.com/kyma-project/lifecycle-manager)
# Lifecycle Manager

Kyma is an opinionated set of Kubernetes-based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. Kyma's Lifecycle Manager is a tool that manages the lifecycle of these modules in your cluster.

## Modularization

Lifecycle Manager was introduced along with the concept of Kyma modularization. With Kyma's modular approach, you can install just the modules you need, giving you more flexibility and reducing the footprint of your Kyma cluster. Lifecycle Manager manages clusters using the [Kyma](api/v1beta2/kyma_types.go) custom resource (CR). The CR defines the desired state of modules in a cluster. With the CR you can enable and disable modules. Lifecycle Manager installs or uninstalls modules and updates their statuses. For more details, read about the [modularization concept in Kyma](https://github.com/kyma-project/community/tree/main/concepts/modularization).

## Basic Concepts

See the list of basic concepts relating to Lifecycle Manager to understand its workflow better.

- Kyma custom resource (CR) - represents Kyma installation in a cluster. It contains the list of modules and their state.
- ModuleTemplate CR - contains modules' metadata with links to their images and manifests. ModuleTemplate CR represents a module in a particular version. Based on this resource Lifecycle Manager enables or disables modules in your cluster.
- Manifest CR - represents resources that make up a module and are to be installed by Lifecycle Manager. The Manifest CR is a rendered module enabled on a particular cluster.
- Module CR, such as Keda CR - allows you to configure the behavior of a module. This is a per-module CR.

For the worklow details, read the [Architecture](docs/technical-reference/architecture.md) document.

## Quick Start

Follow this quick start guide to set up the environment and use Lifecycle Manager to add modules.

### Prerequisites

To use Lifecycle Manager in a local setup, install the following:

- [k3d](https://k3d.io/)
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/)
- [Kyma CLI](https://kyma-project.io/docs/kyma/latest/04-operation-guides/operations/01-install-kyma-CLI)

### Steps

1. To set up the environment, provision a local k3d cluster and install Kyma. Run:

  ```bash
  k3d registry create kyma-registry --port 5001
  k3d cluster create kyma --kubeconfig-switch-context -p 80:80@loadbalancer -p 443:443@loadbalancer --registry-use kyma-registry
  kubectl create ns kyma-system
  kyma alpha deploy
  ```

2. Apply a ModuleTemplate CR. Run the following kubectl command:

  ```bash
  kubectl apply -f {MODULE_TEMPLATE.yaml}
  ```

**TIP:** You can use any deployment-ready ModuleTemplates, such as [cluster-ip](https://github.com/pbochynski/) or [keda](https://github.com/kyma-project/keda-manager).

3. Enable a module. Run:

  ```bash
  kyma alpha add module {MODULE_NAME}
  ```

**TIP:** Check the [modular Kyma interactive tutorial](https://killercoda.com/kyma-project/scenario/modular-kyma) to play with enabling and disabling Kyma modules in both terminal and Busola.

## Read More

Go to the [`Table of Contents`](/docs/README.md) in the `/docs` directory to find the complete list of documents on Lifecycle Manager. Read those to learn more about Lifecycle Manager and its functionalities.

## Contributing

See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct

See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](./LICENSE) file.
