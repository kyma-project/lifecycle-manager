# Lifecycle Manager

<!-- markdown-link-check-disable-next-line -->
[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/lifecycle-manager)](https://api.reuse.software/info/github.com/kyma-project/lifecycle-manager)

## Overview

[Kyma](https://kyma-project.io/) is an opinionated set of Kubernetes-based modular building blocks that provides enterprise-grade capabilities for developing and running cloud-native applications. As an actively maintained open-source project supported by SAP, Kyma serves as the foundation for SAP BTP, Kyma runtime within the SAP Business Technology Platform (BTP).

The Kyma Lifecycle Manager (KLM) is a crucial component at the core of the managed Kyma runtime. Operating within the Kyma Control Plane (KCP) cluster, KLM manages the lifecycle of Kyma modules in the SAP BTP Kyma Runtime (SKR) clusters. These SKR clusters are hyperscaler clusters provisioned for users of the managed Kyma runtime.

KLM's key responsibilities include:
- Installing required Custom Resource Definitions (CRDs) for Kyma module deployment
- Synchronizing the catalog of available Kyma modules to SKR clusters
- Installing, updating, reconciling, and deleting Kyma module resources in SKR clusters
- Watching SKR clusters for changes requested by the users

KLM is built using the [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework and extends the Kubernetes API through custom resource definitions. For detailed information about these resources, see [Lifecycle Manager Resources](./docs/contributor/resources/README.md).

## Usage

If you are a Kyma end user, see the [user documentation](./docs/user/README.md).

## Development

If you want to provide new features for Lifecycle Manager, develop a module, or are part of the SRE team, visit the [contributor](/docs/contributor/) folder. You will find the following documents:

* [Architecture](/docs/contributor/01-architecture.md)
* [Controllers](/docs/contributor/02-controllers.md)
* [Provide Credentials for Private OCI Registry Authentication](/docs/contributor/03-config-private-registry.md)
* [Configure a Local Test Setup](/docs/contributor/04-local-test-setup.md)
* [API Changelog](/docs/contributor/05-api-changelog.md)
* [Resources](/docs/contributor/resources/README.md)
  * [Kyma](/docs/contributor/resources/01-kyma.md)
  * [Manifest](/docs/contributor/resources/02-manifest.md)
  * [ModuleTemplate](/docs/contributor/resources/03-moduletemplate.md)
  * [Watcher](/docs/contributor/resources/04-watcher.md)
  * [ModuleReleaseMeta](/docs/contributor/resources/05-modulereleasemeta.md)

## Contributing

See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct

See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](./LICENSE) file.
