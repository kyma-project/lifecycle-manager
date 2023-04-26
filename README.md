# Lifecycle Manager

Kyma is an opinionated set of Kubernetes-based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. Kyma's Lifecycle Manager is a tool that manages the lifecycle of these modules in your cluster.

## Modularization

<!-- moved to [docs/modularization.md](docs/modularization.md) -->

## Quick Start

Follow this quick start guide to set up the environment and use Lifecycle Manager to enable modules.

### Prerequisites

To use Lifecycle Manager in a local setup, install the following:

- [k3d](https://k3d.io/)
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/)
- [Kyma CLI](https://kyma-project.io/docs/kyma/latest/04-operation-guides/operations/01-install-kyma-CLI)

### Steps

1. To set up the enviroment, provision a local k3d cluster and install Kyma. Run:

  ```bash
  kyma provision k3d
  kyma alpha deploy
  ```

2. Apply a module template. Run the following kubectl command:

  ```bash
  kubectl apply -f cluster-ip-module-template-beta.yaml
  ```

3. Enable a module.

  ```bash
  kyma alpha enable module cluster-ip
  ```

**TIP:** Check the [modular Kyma interactive tutorial](https://killercoda.com/kyma-project/scenario/modular-kyma) to play with enabling and disabling Kyma modules in both terminal and Busola.

<!-- If you are new to our Lifecycle Manager and want to get started quickly, we recommend that you follow our [Quick Start Guide](./docs/user/quick-start.md). This guide will walk you through the basic steps of setting up your local KCP cluster to installing the Lifecycle Manager, and using the main features. ??? -->

## Deployment / Delivery models

lifecycle-manager (and module operators) can run in 2 modes:

- in-cluster - regular deployment in the kubernetes cluster where kyma should be deployed, control-plane manages itself
- control-plane - deployment on central kubernetes cluster that manages multiple kyma installations remotely (installing kyma on the remote clusters based on a secret providing connectivity details)

Which mode is used is based on the `.spec.target` attribute in the `ModuleTemplate`, determining wether a Module needs to be installed in the remote cluster or not.

They both target different use cases. While in-cluster mode is useful for classical deployment of kyma with 1 cluster in play, the general consensus is that for large scale operations, it is recommended to either use an aggregated API-Server or use Clusters to manage other Clusters (nowadays known as Control-Plane)

This means that, depending on your environment you might be running lifecycle-manager in one or the other mode.

For local development, as well as for testing and verification purposes in integration testing, we recommend to use single-cluster mode. For E2E Testing,
and testing of scalability as well as remote reconciliation, we recommend the use of a separate control-plane cluster.

### Release Lifecycles for Modules 

Teams providing module operators should work (and release) independently from lifecycle-manager. In other words, lifecycle-manager should not have hard-coded dependencies to any module operator. 
As such, all module interactions are abstracted through the [ModuleTemplate](api/v1beta1/moduletemplate_types.go).

This abstraction of a template is used for generically deploying instances of a module within a Kyma Runtime at a specific Release Group we call `Channel` (for more information, visit the respective Chapter in the [Concept for Modularization](https://github.com/kyma-project/community/tree/main/concepts/modularization#release-channels)). It contains not only a specification of a Module with it's different components through [OCM Component Descriptors](https://github.com/gardener/component-spec/blob/master/doc/proposal/02-component-descriptor.md).

These serve as small-scale BOM's for all contents included in a module and can be interpreted by Lifecycle Manager and [Module Manager](https://github.com/kyma-project/module-manager/)
to correctly install a module. (for more information, please have a look at the respective chapter in the [Kyma Modularization Concept](https://github.com/kyma-project/community/tree/main/concepts/modularization#component-descriptor))

### Versioning and Releasing

Kyma up to Version 2.x was always a single release. However, the vision of lifecycle-manager is to fully encapsulate individual Modules, with each providing a (possibly fully independent) Release Cycle.
However, Control-Plane deliveries are by design continuously shipped and improved. As a result, even if we will continue to support versioned Module Deliveries, the Lifecycle-Manager and its adjacent infrastructure will be maintained and delivered continously and it is recommended to track upstream as close as possible.

### Comparison to the Old Reconciler

Traditionally, Kyma was installed with the [Kyma Reconciler](https://github.com/kyma-incubator/reconciler), a Control-Plane implementation of our architecture based on polling and a SQL Store for tracking reconciliations.
While this worked great for smaller and medium scale deliveries, we had trouble to scale and maintain it when put under significant load.
We chose to replace this with Operator-focused Reconciliation due to various reasons, more details on the reasoning can be found in our [Concept for Operator Reconciliation](https://github.com/kyma-project/community/tree/main/concepts/operator-reconciliation)

## Testing and implementation guide for Lifecycle Manager developers

- For our detailed overview and reference documentation refer to our [reference overview](docs/developer/reference-documentation.md)
- For a detailed cluster and module setup refer to our [test environment guide](docs/developer/local-test-setup.md)
- For configuring the lifecycle-manager operator refer to our [developer guide](docs/user/starting-operator-with-webhooks.md)
