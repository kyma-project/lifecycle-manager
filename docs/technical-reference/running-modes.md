# Running Modes

Lifecycle Manager can run in two modes:

* **single-cluster:** Deployment mode in which Lifecycle Manager is running on the same cluster in which you deploy Kyma. This mode doesn't require [synchronization](api/README.md#synchronization-of-module-catalog-with-remote-clusters) of Kyma CRs or ModuleTemplate CRs.
* **control-plane:** Deployment mode in which Lifecycle Manager is running on the central Kubernetes cluster that manages multiple remote clusters which are targets for Kyma installations. In this mode, Kyma and ModuleTemplate CRs are synchronized between the central cluster and remote ones. Access to remote clusters is enabled using centrally-managed K8s Secrets with the required connection configuration.

To configure the running mode for Lifecycle Manager, use the `in-kcp-mode` command-line flag. By default, the flag is set to `false`. If set to `true`, Lifecycle Manager runs in the control-plane mode.

> Tip: Use the single-cluster mode for local development and testing. For E2E testing, testing of scalability and remote reconciliation, we recommend to use a separate Control Plane cluster.

## Release Lifecycles for Modules

Teams providing module operators should work (and release) independently of Lifecycle Manager. In other words, Lifecycle Manager should not have hard-coded dependencies to any module operator.
As such, all module interactions are abstracted through the [ModuleTemplate](/api/v1beta2/moduletemplate_types.go).

This abstraction of a module template is used for generically deploying instances of a module within a Kyma Runtime at a specific Release Group we call `Channel` (for more information, visit the respective Chapter in the [Concept of Modularization](https://github.com/kyma-project/community/tree/main/concepts/modularization#release-channels)). It not only contains a specification of a Module with its underlying components through [OCM Component Descriptors](https://github.com/gardener/component-spec/blob/master/doc/proposal/02-component-descriptor.md), but also talks in detail about the schemas, labels, and other essential resources.

These serve as small-scale BoM's for all contents included in a module and can be interpreted by Lifecycle Manager and [Module Manager](https://github.com/kyma-project/module-manager/)
to correctly install a module (for more information, please have a look at the respective chapter in the [Kyma Modularization Concept](https://github.com/kyma-project/community/tree/main/concepts/modularization#component-descriptor)).

## Versioning and Releasing

Kyma up to Version 2.x was always a single release. However, the vision of Lifecycle Manager is to fully encapsulate individual modules, with each providing a (possibly fully independent) release cycle.
By design, the KCP deliveries are continuously shipped and improved. We aim to support versioned module deliveries, so the Lifecycle Manager and its adjacent infrastructure will be maintained as well as delivered continuously, and it is recommended to track upstream as close as possible.

## Comparison to the Old Reconciler

Traditionally, Kyma was installed with the [Kyma Reconciler](https://github.com/kyma-incubator/reconciler), a Control-Plane implementation of our architecture based on polling and a SQL Store for tracking reconciliations.
While this worked great for smaller and medium scale deliveries, we had trouble to scale and maintain it when put under significant load.
We chose to replace this with operator-focused reconciliation - for details on the reasoning, read [Concept for Operator Reconciliation](https://github.com/kyma-project/community/tree/main/concepts/operator-reconciliation).
