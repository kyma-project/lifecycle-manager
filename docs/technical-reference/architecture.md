# Architecture

The architecture of Lifecycle Manager is based on Kubernetes controllers and operators. Lifecycle Manager is a meta operator that coordinates and tracks the lifecycle of Kyma components by delegating it to module operators.

You can compare it with [Operator Lifecycle Manager](https://olm.operatorframework.io/) from Operator Framework. One of the main differences, however, is that the scope of Kyma Lifecycle Manager is to reconcile not only locally but also remote clusters.

Lifecycle Manager:

- manages operators free of dependency trees
- reconciles many clusters in Kyma Control Plane (KCP) at a time
- centralizes the effort on managed Runtimes by providing the reconciliation mechanism
- uses the release channels concept to manage operators delivery

The diagram shows a sample deployment of KCP in interaction with a Kyma runtime.

![Lifecycle Manager Architecture](/docs/assets/lifecycle-manager-architecture.svg)

To run, Lifecycle Manager uses the following workflow:

1. Each module consists of its manager and custom resource. For example, Keda Manager and a Keda CR represent Keda module.

2. A runtime Admin adds and/or removes modules using a Kyma CR. The Kyma CR repersents Kyma installation on a cluster. It includes a list of installed modules and their statuses. Lifecycle Manager watches the CR and uses the synchronization mechanism to update it on a cluster. Together with the Kyma CR, Lifecycle Manager reads also the kubeconfig Secret to access the Kyma Runtime.

3. To manage a module, Lifecycle Manager requires a ModuleTemplate CR. ModuleTemplate CR contains module's metadata. It represents a module in a particular version. All ModuleTemplate CRs exist in Kyma Control Plane which is the central cluster with Kyma infrastructure. Lifecycle Manager uses those ModuleTemplate CRs to create a Module Catalog with ModuleTemplate CRs available for a particluar Kyma rutime. Lifecycle Manager creates the Module Catalog based on labels, such as `internal`, or `beta`, and uses the synchronization mechanism to update the the Module Catalog porfolio.

4. Lifecycle Manager reads a ModuleTemplate CR and creates a Manifest CR. The Manifest CR represents resources that make up a module and are to be installed by Lifecycle Manager. The Manifest CR is a rendered module installed on a particular cluster.

## Controllers

Apart from the custom resources, Lifecycle Manager uses also Kyma, Manifest and Watcher controllers:

- [Kyma controller](../../internal/controller/kyma_controller.go) - reconciles the Kyma CR which means creating Manifest CRs for each Kyma module enabled in the Kyma CR and deleting them when modules are disabled in the Kyma CR. It is also responsible for synchronising ModuleTemplate CRs between KCP and Kyma runtimes.
- [Manifest controller](../../internal/controller/manifest_controller.go) - reconciles the Manifest CRs created by the Kyma controller, which means, installing components specified in the Manifest CR on the target SKR cluster and removing them when the Manifest CRs are flagged for deletion.
- [Watcher controller](../../internal/controller/watcher_controller.go) - reconciles the Watcher CR which means creating Istio Virtual Service resources on KCP when a Watcher CR is created and removing the same resources when it is deleted. This is done in order to configure the routing of the messages coming from the watcher agent installed on each Kyma runtime and going to a listener agent deployed on KCP.

For more details about Lifecycle Manager controllers, read the [Controllers](controllers.md) document.

## Read more

The architecture is based on Kubernetes API and resources, and on best practices for building Kubernetes operators. To learn more, read the following:

- [Kubebuilder book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)
- [Best practices for building Kubernetes Operators and stateful apps](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps)
- [Operator SDK - Best Practices](https://sdk.operatorframework.io/docs/best-practices/).
