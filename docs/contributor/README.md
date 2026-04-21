# Lifecycle Manager Contributor Documentation

* [Architecture](01-architecture.md)
* [Lifecycle Manager Controllers](02-controllers.md)
* [Provide Credentials for Private OCI Registry Authentication](03-config-private-registry.md)
* [Configure a Local Test Setup (VS Code & GoLand)](04-local-test-setup.md)
* [API Changelog](05-api-changelog.md)
* [Lifecycle Manager Resources](resources/README.md)
  * [Kyma](resources/01-kyma.md)
  * [Manifest](resources/02-manifest.md)
  * [ModuleTemplate](resources/03-moduletemplate.md)
  * [Watcher](resources/04-watcher.md)
  * [ModuleReleaseMeta](resources/05-modulereleasemeta.md)
* [Synchronization Between Kyma Control Plane and SAP BTP, Kyma Runtime](08-kcp-skr-synchronization.md)
* [Lifecycle Manager Metrics](09-metrics.md)
* [Maintenance Windows](10-maintenance-windows.md)
* [Lifecycle Manager Components](11-components.md)
* [Lifecycle Manager Flags](12-klm-arguments.md)
* [Creating ModuleTemplate(using modulectl & ocm cli)](14-creating-moduletemplate.md)
* [Notable Changes](15-notable-changes.md)

## Contributing to Documentation for Private and Partner-Managed Landscapes Operators

If you update, add, or remove content in [SAP BTP, Kyma Runtime Operator's Guide](https://help.sap.com/docs/KYMAOPS/kyma_product-kyma-runtime_kyma-operator-guide/00-05-sap-btp-kyma-runtime-operators-guide-intro.html?state=DRAF), that is meant for the operators and administrators of private and partner-managed landscapes, follow these steps:

1. Make sure that the source file or directory is part of the [`manifest.yaml`](https://github.tools.sap/kyma/product-kyma-runtime/blob/main/manifest.yaml) file.
2. Make sure that all the relevant, operator-related documents are part of the [`toc.yaml`](https://github.tools.sap/kyma/product-kyma-runtime/blob/main/docs/toc.yaml) file.
