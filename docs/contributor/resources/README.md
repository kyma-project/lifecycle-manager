# Lifecycle Manager Resources

The API of Lifecycle Manager is based on Kubernetes Custom Resource Definitions (CRDs), which extend the Kubernetes API with custom additions. The CRDs allow Lifecycle Manager to manage clusters and modules. To inspect the specification of the Lifecycle Manager resources, see:

* [Kyma CRD](01-kyma.md)
* [Manifest CRD](02-manifest.md)
* [ModuleTemplateCRD](03-moduletemplate.md)
* [Watcher CRD](04-watcher.md)

## Synchronization of Module Catalog with Remote Clusters

Lifecycle Manager ensures the Module Catalog is correctly synchronized with users' runtimes.
The Module Catalog consists of all modules, represented by ModuleTemplates CR, that are available for a user. The Module Catalog portfolio may vary for different users.
The synchronization mechanism described below is essential to allow users to enable modules in their clusters.
The mechanism is controlled by the set of labels that are configured on Kyma and ModuleTemplate CRs in the Control Plane. The labels are: `operator.kyma-project.io/sync`, `operator.kyma-project.io/internal`, and `operator.kyma-project.io/beta`.
The v1beta2 API introduces three groups of modules:

* Default modules, synchronized by default.
* Internal modules, synchronized per-cluster only if configured explicitly on the corresponding Kyma CRs. To mark a ModuleTemplate CR as `internal`, use the `operator.kyma-project.io/internal` label and set it to `true`.
* Beta modules, synchronized per-cluster only if configured explicitly on the corresponding Kyma CRs. To mark a ModuleTemplate CR as `beta`, use the `operator.kyma-project.io/beta` label and set it to `true`.

By default, without any labels configured on Kyma and ModuleTemplate CRs, a ModuleTemplate CR is synchronized with remote clusters.

**NOTE:** The ModuleTemplate CRs synchronization is enabled only when Lifecycle Manager runs in the control-plane mode. Lifecycle Manager running in the single-cluster mode, doesn't require any CR synchronization.

**NOTE:** Disabling synchronization for already synchronized ModuleTemplates CRs doesn't remove them from remote clusters. The CRs remain as they are, but any subsequent changes to these ModuleTemplate CRs in the Control Plane are not synchronized.

For more information, see [the Kyma CR synchronization labels](./01-kyma.md#operatorkyma-projectio-labels) and [the ModuleTemplate CR synchronization labels](./03-moduletemplate.md#operatorkyma-projectio-labels).

## Stability

See the list of CRs involved in Lifecycle Manager's workflow and their stability status:

| Version | Name                                                | Stability                                                                                                                                                                                                    |
|:--------|-----------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| v1beta2 | [Kyma](/api/v1beta2/kyma_types.go)                         | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [ModuleTemplate](/api/v1beta2/moduletemplate_types.go)     | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Manifest](/api/v1beta2/manifest_types.go)                 | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Watcher](/api/v1beta2/watcher_types.go)                   | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |

For more information on changes introduced by an API version, see [API Changelog](../05-api-changelog.md).
