# Lifecycle Manager

<!-- markdown-link-check-disable-next-line  -->
[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/lifecycle-manager)](https://api.reuse.software/info/github.com/kyma-project/lifecycle-manager)

## Overview

Lifecycle Manager is an operator based on the [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework. It extends Kubernetes API by providing multiple Custom Resource Definitions, which allow you to manage their resources through custom resources (CR). For more information, see [Lifecycle Manager Resources](./docs/contributor/resources/README.md).

Lifecycle Manager manages the lifecycle of [Kyma Modules](https://help.sap.com/docs/btp/sap-business-technology-platform/kyma-modules) in a cluster. It was introduced along with the concept of Kyma [modularizaion](https://github.com/kyma-project/community/tree/main/concepts/modularization).

For more information on the Lifecycle Manager's workflow, see the [Architecture](docs/contributor/01-architecture.md) document.

## Usage

If you are a Kyma end user, see the [user documentation](./docs/user/README.md).

## Development

If you want to provide new features for Lifecycle Manager, develop a module, or are part of the SRE team, visit the [contributor](/docs/contributor/) folder. You will find the following documents:

* [Architecture](/docs/contributor/01-architecture.md)
* [Controllers](/docs/contributor/02-controllers.md)
* [Provide Credentials for Private OCI Registry Authentication](/docs/contributor/03-config-private-registry.md)
* [Local Test Setup in the Control Plane Mode Using k3d](/docs/contributor/04-local-test-setup.md)
* [Resources](/docs/contributor/resources/README.md)
  * [Kyma](/docs/contributor/resources/01-kyma.md)
  * [Manifest](/docs/contributor/resources/02-manifest.md)
  * [ModuleTemplate](/docs/contributor/resources/03-moduletemplate.md)
  * [Watcher](/docs/contributor/resources/04-watcher.md)

## Contributing

See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct

See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](./LICENSE) file.
