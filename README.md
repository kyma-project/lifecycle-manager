# Lifecycle Manager

<!-- markdown-link-check-disable-next-line -->
[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/lifecycle-manager)](https://api.reuse.software/info/github.com/kyma-project/lifecycle-manager)

## Overview

[Kyma](https://kyma-project.io/) is an opinionated set of Kubernetes-based modular building blocks that provides enterprise-grade capabilities for developing and running cloud-native applications. As an actively maintained open-source project supported and managed by SAP, Kyma serves as the foundation for SAP BTP, Kyma runtime within the SAP Business Technology Platform (BTP).

Kyma Lifecycle Manager (KLM) is a crucial component at the core of SAP BTP, Kyma runtime. For more information, see [Lifecycle Manager](/docs/README.md) in the `/docs` directory.

## Usage

If you are a Kyma end user, see the [user documentation](/docs/user/README.md).

## Development

If you want to provide new features for Lifecycle Manager, develop a module, or are part of the SRE team, visit the [contributor](/docs/contributor/) and the [operator](/docs/operator/operator-index.md) folders. You will find the following documents:

* [Architecture](/docs/contributor/01-architecture.md)
* [Lifecycle Manager Controllers](/docs/contributor/02-controllers.md)
* [Provide Credentials for Private OCI Registry Authentication](/docs/contributor/03-config-private-registry.md)
* [Configure a Local Test Setup (VS Code & GoLand)](/docs/contributor/04-local-test-setup.md)
* [API Changelog](/docs/contributor/05-api-changelog.md)
* [Lifecycle Manager Resources](./docs/contributor/resources/README.md)
  * [Kyma](/docs/contributor/resources/01-kyma.md)
  * [Manifest](/docs/contributor/resources/02-manifest.md)
  * [ModuleTemplate](/docs/contributor/resources/03-moduletemplate.md)
  * [Watcher](/docs/contributor/resources/04-watcher.md)
  * [ModuleReleaseMeta](/docs/contributor/resources/05-modulereleasemeta.md)
* [New Module Submission and Promotion Concept](/docs/contributor/06-module-migration-concept.md)
* [New Module Submission and Promotion Process: Migration Guide](/docs/contributor/07-module-migration-guide.md)
* [Synchronization Between Kyma Control Plane and SAP BTP, Kyma Runtime](/docs/contributor/08-kcp-skr-synchronization.md)
* [Lifecycle Manager Metrics](/docs/contributor/09-metrics.md)
* [Maintenance Windows](/docs/contributor/10-maintenance-windows.md)
* [Lifecycle Manager Components](/docs/contributor/11-components.md)
* [Lifecycle Manager Flags](/docs/contributor/12-klm-arguments.md)

## Contributing

See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct

See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](LICENSE) file.
