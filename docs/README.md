# Lifecycle Manager

Kyma Lifecycle Manager (KLM) is a crucial component at the core of the managed Kyma runtime, called SAP BTP, Kyma runtime. Operating within the Kyma Control Plane (KCP) cluster, KLM manages the lifecycle of Kyma modules in the SAP BTP, Kyma Runtime (SKR) clusters. These SKR clusters are hyperscaler clusters provisioned for users of the managed Kyma runtime.

KLM's key responsibilities include:

* Installing Custom Resource Definitions (CRDs) required for Kyma module deployment
* Synchronizing the catalog of available Kyma modules to SKR clusters
* Installing, updating, reconciling, and deleting Kyma module resources in SKR clusters
* Watching SKR clusters for changes requested by the users

KLM is built using the [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework and extends the Kubernetes API through custom resource definitions. For detailed information about these resources, see [Lifecycle Manager Resources](./contributor/resources/README.md).
