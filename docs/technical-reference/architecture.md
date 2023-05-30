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

## Controllers

- [Kyma Controller](../../controllers/kyma_controller.go) - in active development (continuous) - Expect Bugs and fast-paced development of new features
- [Manifest Controller](../../controllers/manifest_controller.go) - directs to the [Declarative Library](../../internal/declarative/v2), a reconciliation library we use to install all modules
- [Watcher Controller](../../controllers/watcher_controller.go) - maintains VirtualService entries for events coming from runtime clusters, mostly stable

## Read more

The architecture is based on Kubernetes API and resources, and on best practices for building Kubernetes operators. To learn more, read the following:

- [Kubebuilder book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)
- [Best practices for building Kubernetes Operators and stateful apps](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps)
- [Operator SDK - Best Practices](https://sdk.operatorframework.io/docs/best-practices/).
