# Lifecycle Manager

Kyma is an opinionated set of Kubernetes-based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. Kyma's Lifecycle Manager is a tool that manages the lifecycle of these modules in your cluster.

## Basic Concepts and Modularization

This list of basic concepts relating to Lifecycle Manager aims to help you understand its workflow better.

- Kyma custom resource (CR) - [short description]
- ModuleTemplate CR - [short description]
- Module CR - [short description]
- Manifest CR - [short description]
- Watcher CR - [short description]

Lifecycle Manager manages clusters using the [Kyma](api/v1beta1/kyma_types.go) custom resource (CR). The CR defines the desired state of modules in a cluster. With the CR you can enable and disable modules. Lifecycle Manager installs, uninstalls, and updates the module status.

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

2. Apply a module template. Run the following kubectl command:

  ```bash
  kubectl apply -f {MODULE_TEMPLATE.yaml}
  ```

3. Enable a module. Run:

  ```bash
  kyma alpha enable module {MODULE_NAME}
  ```

**TIP:** Check the [modular Kyma interactive tutorial](https://killercoda.com/kyma-project/scenario/modular-kyma) to play with enabling and disabling Kyma modules in both terminal and Busola.

<!-- If you are new to our Lifecycle Manager and want to get started quickly, we recommend that you follow our [Quick Start Guide](./docs/user/quick-start.md). This guide will walk you through the basic steps of setting up your local KCP cluster, installing the Lifecycle Manager, and using the main features. ??? -->

To learn more about Lifecycle Manager and its components, go to the [`docs`](/docs/) directory.
