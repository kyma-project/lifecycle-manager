# Lifecycle Manager

Kyma is an opinionated set of Kubernetes-based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. Kyma's Lifecycle Manager is a tool that manages the lifecycle of these modules in your cluster.

## Modularization

Lifecycle Manager was introduced along with the concept of Kyma modularization. With Kyma's modular approach, you can install just the modules you need, giving you more flexibility and reducing the footprint of your Kyma cluster. Lifecycle Manager manages clusters using the [Kyma](api/v1beta1/kyma_types.go) custom resource (CR). The CR defines the desired state of modules in a cluster. With the CR you can enable and disable modules. Lifecycle Manager installs or uninstalls modules and updates their statuses. For more details, read about the [modularization concept in Kyma](https://github.com/kyma-project/community/tree/main/concepts/modularization).

## Basic Concepts

See the list of basic concepts relating to Lifecycle Manager to understand its workflow better.

- Kyma custom resource (CR) - represents Kyma installation in a cluster. It contains the list of modules and their state.
- ModuleTemplate CR - contains modules' metadata with links to their images and manifests. Based on this resource you can enable or disable modules in your cluster.
- Module CR - allows you to configure the behavior of the module. This is a per-module CR.

## Quick Start

Follow this quick start guide to set up the environment and use Lifecycle Manager to enable modules.

### Prerequisites

To use Lifecycle Manager in a local setup, install the following:

- [k3d](https://k3d.io/)
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/)
- [Kyma CLI](https://kyma-project.io/docs/kyma/latest/04-operation-guides/operations/01-install-kyma-CLI)

### Steps

1. To set up the environment, provision a local k3d cluster and install Kyma. Run:

  ```bash
  kyma provision k3d
  kyma alpha deploy
  ```

2. Apply a ModuleTemplate CR. Run the following kubectl command:

  ```bash
  kubectl apply -f {MODULE_TEMPLATE.yaml}
  ```

**TIP:** You can use any deployment-ready ModuleTemplates, such as [cluster-ip](https://github.com/pbochynski/) or [keda](https://github.com/kyma-project/keda-manager).

3. Enable a module. Run:

  ```bash
  kyma alpha enable module {MODULE_NAME}
  ```

**TIP:** Check the [modular Kyma interactive tutorial](https://killercoda.com/kyma-project/scenario/modular-kyma) to play with enabling and disabling Kyma modules in both terminal and Busola.

<!-- If you are new to our Lifecycle Manager and want to get started quickly, we recommend that you follow our [Quick Start Guide](./docs/user/quick-start.md). This guide will walk you through the basic steps of setting up your local KCP cluster, installing the Lifecycle Manager, and using the main features. ??? -->

## Read More

Go to the [`Table of Contents`](/docs/README.md) in the `/docs` directory to find the complete list of documents on Lifecycle Manager. Read those to learn more about Lifecycle Manager and its functionalities.
