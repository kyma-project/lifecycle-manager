# Architecture

The architecture of Lifecycle Manager is based on Kubernetes controllers and operators. Lifecycle Manager is a meta operator that coordinates and tracks the lifecycle of Kyma components by delegating it to module operators.

You can compare it with [Operator Lifecycle Manager](https://olm.operatorframework.io/) from Operator Framework. One of the main differences, however, is that the scope of Kyma Lifecycle Manager is to reconcile not only locally but also remote clusters.

Lifecycle Manager:

* manages a set of independent operators
* reconciles many remote clusters at a time while operating in Kyma Control Plane (KCP)
* uses the release channels concept to manage operators' delivery

The diagram shows a sample deployment of KCP in interaction with a Kyma runtime.

![Lifecycle Manager Architecture](./assets/lifecycle-manager-architecture.svg)

To run, Lifecycle Manager uses the following workflow:

1. Each module consists of its manager and custom resource. For example, Keda Manager and a Keda CR represent Keda module.

2. A runtime Admin adds and/or removes modules using a Kyma CR. The Kyma CR represents Kyma installation on a cluster. It includes a list of installed modules and their statuses. Lifecycle Manager watches the CR and uses the synchronization mechanism to update it on a cluster. Together with the Kyma CR, Lifecycle Manager reads also the kubeconfig Secret to access the Kyma runtime.

3. To manage a module, Lifecycle Manager requires its definition and version-related metadata. The ModuleTemplate and ModuleReleaseMeta CRs represent the definition and version-related metadata for a module. The ModuleTemplate CR represents a module in a particular version, while the ModuleReleaseMeta CR describes the mapping between module versions and available channels. All ModuleTemplate CRs, along with their related ModuleReleaseMeta CRs, exist in Kyma Control Plane which is the central cluster with Kyma infrastructure. The set of ModuleTemplate CRs and related ModuleReleaseMeta CRs available for a particular Kyma runtime is called the Module Catalog. Lifecycle Manager creates the Module Catalog based on labels, such as `internal`, or `beta`, and uses the synchronization mechanism to update the Module Catalog portfolio.

4. Lifecycle Manager uses ModuleReleaseMeta and ModuleTemplate CRs to read a module's definition and create a Manifest CR. The Manifest CR represents resources that make up a module and are to be installed on a remote cluster by Lifecycle Manager.

5. Lifecycle Manager reconciles, namely watches and updates, a set of resources that make up a module. This process lasts until a module is listed in the remote cluster Kyma CR.

## Controllers

Apart from the custom resources, Lifecycle Manager uses also Kyma, Manifest, and Watcher controllers:

* [Kyma controller](./02-controllers.md#kyma-controller) - reconciles the Kyma CR which means creating Manifest CRs for each Kyma module enabled in the Kyma CR and deleting them when modules are disabled in the Kyma CR. It is also responsible for synchronising ModuleTemplate CRs between KCP and Kyma runtimes.
* [Manifest controller](./02-controllers.md#manifest-controller) - reconciles the Manifest CRs created by the Kyma controller, which means, installing components specified in the Manifest CR in the target SKR cluster and removing them when the Manifest CRs are flagged for deletion.
* [Mandatory Modules controller](02-controllers.md#mandatory-modules-controllers) - reconciles the mandatory ModuleTemplate CRs that have the `operator.kyma-project.io/mandatory-module` label, selecting the highest version if duplicates exist. It translates the ModuleTemplate CRs to Manifest CRs linked to the Kyma CR, ensuring changes propagate. For removal, a deletion controller marks the related Manifest CRs, removes finalizers, and deletes the ModuleTemplate CR.
* [Purge controller](./02-controllers.md#purge-controller) - reconciles the Kyma CRs that are marked for deletion longer than the grace period, which means purging all the resources deployed by Lifecycle Manager in the target SKR cluster.
* [Watcher controller](./02-controllers.md#watcher-controller) - reconciles the Watcher CR which means creating Istio Virtual Service resources in KCP when a Watcher CR is created and removing the same resources when the Watcher CR is deleted. This is done to configure the routing of the messages that come from the watcher agent, installed on each Kyma runtime, and go to a listener agent deployed in KCP.
* [Istio Gateway Secret controller](./02-controllers.md#istio-gateway-secret-controller) - reconciles the Istio gateway certificate secret.

For more details about Lifecycle Manager controllers, read the [Controllers](./02-controllers.md) document.

## Read More

The architecture is based on Kubernetes API and resources, and on best practices for building Kubernetes operators. To learn more, read the following:

* [Kubebuilder](https://kubebuilder.io/)
* [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)
* [Best practices for building Kubernetes Operators and stateful apps](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps)
* [Operator SDK - Best Practices](https://sdk.operatorframework.io/docs/best-practices/).
