# Lifecycle Manager

Kyma Lifecycle Manager (KLM) is a core component of the managed offering SAP BTP, Kyma runtime ("SKR"). Operating within the Kyma Control Plane (KCP) cluster, KLM manages the lifecycle of Kyma modules in SKR clusters. These SKR clusters are hyperscaler clusters provisioned for SKR users.

KLM takes care of the following tasks:

* Installing Custom Resource Definitions (CRDs) required for Kyma module deployment
* Synchronizing the catalog of available Kyma modules to SKR clusters
* Installing, updating, reconciling, and deleting Kyma module resources in SKR clusters
* Watching SKR clusters for changes requested by the users

KLM is built using the [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework and extends the Kubernetes API through custom resource definitions. For detailed information about these resources, see [Lifecycle Manager Resources](./contributor/resources/README.md).

## Next Steps

* If you're an end user of SAP BTP, Kyma runtime, go to the [`user`](/docs/user/README.md) directory.
* If you're a developer interested in the module's code, go to the [`contributor`](/docs/contributor/README.md) directory.
* If you're a Lifecycle Manager operator, go to the [`operator`](/docs/operator/README.md) directory.
