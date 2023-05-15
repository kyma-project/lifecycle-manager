# Architecture

The architecture of Lifecycle Manager is based on Kubernetes controllers and operators. Lifecycle Manager is a meta operator that coordinates and tracks the lifecycle of Kyma components by delegating it to module operators.

You can compare it with [Operator Lifecycle Manager](https://olm.operatorframework.io/) from Operator Framework, as we are strongly inspired by their ideas. One of the main differences, however, is that the scope of Kyma Lifecycle Manager is to reconcile not only locally but also remote clusters.

Lifecycle Manager:

- manages operators free of dependency trees
- reconciles many clusters in Kyma Control Plane at a time
- centralizes the effort on managed Runtimes by providing the reconciliation mechanism
- uses the familiar [release channels concept](link?) to manage operators delivery

The architecture is based on Kubernetes API and resources, and on best practices for building Kubernetes operators. To learn more, read the following:
- [Kubebuilder book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)
- [Best practices for building Kubernetes Operators and stateful apps](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps)
- [Operator SDK - Best Practices](https://sdk.operatorframework.io/docs/best-practices/).

The diagram shows a sample deployment of Kyma Control Plane in interaction with a Kyma runtime.

**CAUTION:** Please note that real deliveries can significantly differ from those presented in the diagram depending on the tradeoffs chosen for reconciliation.

![Kyma Operator Architecture](/docs/assets/kyma-operator-architecture.svg)

## Stability

Some architecture decisions were derived from business requirements or proofs of concepts and are still
subject to change. However, the general reconciliation model is ready to use.

See the list of components involved in Lifecycle Manager's workflow and their stability status:

| Version | System Component                                                | Stability                                                                                                                                                                                                    |
|:--------|-----------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| v1beta2 | [Kyma](../../api/v1beta2/kyma_types.go)                         | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [ModuleTemplate](../../api/v1beta2/moduletemplate_types.go)     | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Manifest](../../api/v1beta2/manifest_types.go)                 | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Watcher](../../api/v1beta2/watcher_types.go)                   | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
|         | [Kyma Controller](../../controllers/kyma_controller.go)         | In active development (continuous) - Expect Bugs and fast-paced development of new features                                                                                                                  |
|         | [Manifest Controller](../../controllers/manifest_controller.go) | Directs to the [Declarative Library](../../internal/declarative/v2), a reconciliation library we use to install all modules                                                                                  |
|         | [Watcher Controller](../../controllers/watcher_controller.go)   | Maintains VirtualService entries for events coming from runtime clusters, mostly stable                                                                                                                      |
