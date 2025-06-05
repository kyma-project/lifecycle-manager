# Lifecycle Manager Resources

The API of Lifecycle Manager is based on Kubernetes Custom Resource Definitions (CRDs), which extend the Kubernetes API with custom additions. The CRDs allow Lifecycle Manager to manage clusters and modules. To inspect the specification of the Lifecycle Manager resources, see:

* [Kyma CRD](01-kyma.md)
* [Manifest CRD](02-manifest.md)
* [ModuleTemplateCRD](03-moduletemplate.md)
* [Watcher CRD](04-watcher.md)
* [ModuleReleaseMeta CRD](05-modulereleasemeta.md)

For information about how the Module Catalog (ModuleTemplate CRs and related ModuleReleaseMetas CRs) and Kyma CR are synchronized between the Kyma Control Plane (KCP) and SAP Kyma Runtime (SKR) clusters, see the [KCP-SKR synchronization documentation](../08-kcp-skr-synchronization.md).

## Stability

See the list of CRs involved in Lifecycle Manager's workflow and their stability status:

| Version | Name                                                | Stability                                                                                                                                                                                                    |
|:--------|-----------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| v1beta2 | [Kyma](/api/v1beta2/kyma_types.go)                         | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [ModuleTemplate](/api/v1beta2/moduletemplate_types.go)     | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Manifest](/api/v1beta2/manifest_types.go)                 | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Watcher](/api/v1beta2/watcher_types.go)                   | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |

For more information on changes introduced by an API version, see [API Changelog](../05-api-changelog.md).
