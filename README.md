# Lifecycle Manager

Kyma is the opinionated set of Kubernetes based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. Kyma's Lifecycle Manager (or `lifecycle-manager` in various technical references) is a tool that manages the lifecycle of these components in your cluster.

### Contents
* [How it works](#how-it-works)
  * [Example](#example)
  * [Getting Started](#getting-started)
* [Architecture](#architecture)
  * [Stability](#stability)
* [Deployment / Delivery models](#deployment--delivery-models)
  * [Release Lifecycles for Modules](#release-lifecycles-for-modules)
  * [Versioning and Releasing](#versioning-and-releasing)
  * [Comparison to the Old Reconciler](#comparison-to-the-old-reconciler)
* [Testing and implementation guide](#testing-and-implementation-guide)

## How it works

Lifecycle Manager manages Clusters through the [Kyma Custom Resource](api/v1beta1/kyma_types.go), which contains a desired state of all modules in a cluster. Imagine it as a one stop shop for a cluster where you can add and remove modules with domain-specific functionality with additional configuration.

The modules themselves are bundled containers based on the [OCI Image Format Specification][https://github.com/opencontainers/image-spec] . They contain an immutable layer set of a module operator and its configuration.
This is installed and controlled by Lifecycle Manager. We use [Open Component Model](https://ocm.software) to describe all of our modules descriptively.

Based on the [ModuleTemplate Custom Resource](api/v1beta1/moduletemplate_types.go), the module is resolved from its individual layers and version and is used as a template for the [Manifest](api/v1beta1/manifest_types.go). Whenever a module is accepted by Lifecycle Manager the ModuleTemplate gets translated into a Manifest, which describes the actual desired state of the module operator.

The Lifecycle Manager then updates the [Kyma Custom Resource](api/v1alpha1/kyma_types.go) of the cluster based on the observed status changes in the Module Custom Resources (similar to a native kubernetes deployment tracking availability).

Module operators only have to watch their own custom resources and reconcile modules in the target clusters to the desired state. 

### Example

A sample `Kyma` CR could look like this:
```
apiVersion: operator.kyma-project.io/v1beta1
kind: Kyma
metadata:
  name: my-kyma
spec:
  modules:
  - name: my-module
```

The creation of the custom resource triggers a reconciliation that
1. looks for a ModuleTemplate based on search criteria, for example the OCM Component Name of the Module or simply the of the `ModuleTemplate`
2. creates a `Manifest` for `my-module` based on a [ModuleTemplate](api/v1beta1/moduletemplate_types.go) found in the cluster by resolving all relevant image layer for the installation
3. installing the contents of the modules operator by applying them to the cluster, and observing its state
4. reporting back all states observed in the `Manifest` which then gets propagated to the `Kyma` resource for the cluster.
   Lifecycle Manager then uses this to aggregate and combine the readiness condition of the cluster and determine the installation state or trigger more reconciliation loops as needed.

As mentioned above, when each module operator completes their installation, it reports its own resource status. However, to accurately report state, we read out the `.status.state` field to accumulate status reporting for an entire cluster.

### Getting Started

To get started, we have prepared a curated reference implementation of an operator in our [Template Operator](https://github.com/kyma-project/template-operator). On top of this we have prepared multiple likely use cases for modules (e.g. implementing an installation for a third-party-module).

In summary, every module follows basic steps that we have accompanied with respective [cli](https://github.com/kyma-project/cli) commands:

1. Create a Kyma Control Plane on a cluster
  ```shell
  kyma alpha deploy
  ```
2. Create a module in a specific semantic version and a fully qualified domain name (FQDN). The easiest way to make 
  ```shell
  kyma alpha create module \
    --name kyma-project.io/module/samples/my-module \
    --version=1.0.0 \
    --registry=europe-west3-docker.pkg.dev/sample-registry/sample-subpath
  ```
  This will also output a `template.yaml` file which you can directly apply to a cluster with
  `kubectl apply -f template.yaml`
3. Enable the module in the cluster with a specific release channel
  ```shell
  kyma alpha enable module --name my-module
  ```
  Enjoy your module in your cluster!
4. Disable the module in the cluster
  ```shell
  kyma alpha disable module --name my-module
  ```
## Architecture

The architecture of this operator is based on Kubernetes controllers/operators. `lifecycle-manager` is a meta operator that coordinates and tracks the lifecycle of kyma components by delegating it to module operators. You can compare it to [Operator Lifecycle Manager](https://olm.operatorframework.io/) from Operator Framework, and we are strongly inspired by their ideas. One of the main differentiating factors however, is that the Scope of the Kyma Lifecycle Manager is to reconcile not only locally, but also into remote Clusters.

A few selected key advantages include:

- Manage Operators completely free of dependency-trees and without opinionation on dependency resolution
- Reconcile many clusters (up to 10.000 per control-plane are measured with our performance tests) from a control-plane
- Centralize the effort on managed Runtimes by providing a Control-Plane style Reconciliation Mechanism
- Use familiar Release Concepts of Release Channels to manage delivery of operators

Before you go further, please make sure you understand concepts of Kubernetes API and resources. Recommended reading:
- [Kubebuilder book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)

The architecture is based as much as possible on best practices for building Kubernetes operators ([1](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps), [2](https://sdk.operatorframework.io/docs/best-practices/)).

The diagram below shows a sample deployment of a Control-Plane in interaction with the Kyma Runtime. Please use this diagram not as a single source of truth (as diagrams like to be treated in README's), but rather a reference for navigation of nomenclature and terms, as real deliveries can differ significantly depending on the tradeoffs chosen for reconciliation.

![Kyma Operator Architecture](docs/assets/kyma-operator-architecture.svg)

### Stability

Some architecture decisions were derived from business requirements and experiments (proof of concepts) and are still
subject to change, however the general reconciliation model is considered ready for use already.

Here is a (somewhat complete) list of the different modules in the system together with their stability:

| Version          | System Component                                          | Stability                                                                                                                                                                                                    |
|:-----------------|-----------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| v1alpha1,v1beta1 | [Kyma](api/v1beta1/kyma_types.go)                         | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1alpha1,v1beta1 | [ModuleTemplate](api/v1beta1/moduletemplate_types.go)     | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1alpha1,v1beta1 | [Manifest](api/v1beta1/manifest_types.go)                 | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1alpha1,v1beta1 | [Watcher](api/v1beta1/watcher_types.go)                   | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
|                  | [Kyma Controller](controllers/kyma_controller.go)         | In active development (continuous) - Expect Bugs and fast-paced development of new features                                                                                                                  |
|                  | [Manifest Controller](controllers/manifest_controller.go) | Directs to the [Declarative Library](pkg/declarative/v2), a reconciliation library we use to install all modules                                                                                             |
|                  | [Watcher Controller](controllers/watcher_controller.go)   | Maintains VirtualService entries for events coming from runtime clusters, mostly stable                                                                                                                      |

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

- For a detailed cluster and module setup refer to our [test environment guide](docs/developer/local-test-setup.md)
- For configuring the lifecycle-manager operator refer to our [developer guide](docs/user/starting-operator-with-webhooks.md)
