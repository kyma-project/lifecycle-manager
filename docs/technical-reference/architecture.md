# Architecture

The architecture of Lifecycle Manager is based on Kubernetes controllers and operators. Lifecycle Manager is a meta operator that coordinates and tracks the lifecycle of Kyma components by delegating it to module operators.

You can compare it with [Operator Lifecycle Manager](https://olm.operatorframework.io/) from Operator Framework. One of the main differences, however, is that the scope of Kyma Lifecycle Manager is to reconcile not only locally but also remote clusters.

Lifecycle Manager:

- manages operators free of dependency trees
- reconciles many clusters in Kyma Control Plane at a time
- centralizes the effort on managed Runtimes by providing the reconciliation mechanism
- uses the release channels concept to manage operators delivery

The diagram shows a sample deployment of Kyma Control Plane in interaction with a Kyma runtime.

![Lifecycle Manager Architecture](/docs/assets/lifecycle-manager-architecture.svg)

To run, Lifecycle Manager uses the following workflow:

1. Each module consists of its manager and custom resource. For example, Keda Manager and a Keda CR represent Keda module.

2. A runtime Admin adds and/or removes modules using a Kyma CR. The Kyma CR repersents Kyma installation on a cluster. It includes a list of installed modules and their statuses. Lifecycle Manager watches the CR and uses the synchronization mechanism to update it on a cluster. Together with Kyma CR, Lifecycle Manager reads also kubeconfig Secret to access the Kyma Runtime.

3. To manage a module, Lifecycle Manager requires a ModuleTemplate CR. ModuleTemplate CR contains module's metadata. It represents a module in a particular version. All ModuleTemplate CRs exist in Kyma Control Plane which is the central cluster with Kyma infrastructure. Lifecycle Manager uses those ModuleTemplate CRs to create a Module Catalog with ModuleTenplate CRs available for a particluar Kyma rutime. Lifecycle Manager creates the Module Catalog basing on labels, such as `internal`, `beta`, etc., and uses the synchronization mechanism to update the the Module Catalog porfolio.

4. Lifecycle Manager reads a ModuleTemplate CR and creates a Manifest CR. The Manifest CR represents resources that make up a module and are to be installed by Lifecycle Manager. The Manifest CR is a rendered module installed on a particular cluster.

## Controllers

Apart from the custom resources, Lifecycle Manager uses also three controllers:

- [Kyma Controller](../../controllers/kyma_controller.go) - in active development (continuous) - Expect Bugs and fast-paced development of new features
- [Manifest Controller](../../controllers/manifest_controller.go) - directs to the [Declarative Library](../../internal/declarative/v2), a reconciliation library we use to install all modules
- [Watcher Controller](../../controllers/watcher_controller.go) - maintains VirtualService entries for events coming from runtime clusters, mostly stable

## Read more

The architecture is based on Kubernetes API and resources, and on best practices for building Kubernetes operators. To learn more, read the following:

- [Kubebuilder book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)
- [Best practices for building Kubernetes Operators and stateful apps](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps)
- [Operator SDK - Best Practices](https://sdk.operatorframework.io/docs/best-practices/).
