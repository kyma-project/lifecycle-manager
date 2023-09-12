# Lifecycle Manager API

## Overview

The Lifecycle Manager API types consist of three major custom resources (CRs). Each CR deals with a specific aspect of reconciling modules into their corresponding states.

1. [Kyma CR](/api/v1beta2/kyma_types.go) that introduces a single entry point CustomResourceDefinition to control a cluster and it's desired state.
2. [Manifest CR](/api/v1beta2/manifest_types.go) that introduces a single entry point CustomResourceDefinition to control a module and it's desired state.
3. [ModuleTemplate CR](/api/v1beta2/moduletemplate_types.go) that contains all reference data for the modules to be installed correctly. It is a standardized desired state for a module available in a given release channel.

Additionally, we maintain the [Watcher CR](/api/v1beta2/watcher_types.go) to define the callback functionality for synchronized remote clusters that allows lower latencies before the Control Plane detects any changes.

## Custom Resource Definitions

Read more about the custom resource definitions (CRDs) in the respective documents:

- [Kyma CR](kyma-cr.md)
- [Manifest CR](manifest-cr.md)
- [ModuleTemplate CR](moduleTemplate-cr.md)

## Synchronization of Module Catalog with remote clusters

Lifecycle Manager ensures the Module Catalog is correctly synchronized with users' runtimes.
The Module Catalog consists of all modules, represented by ModuleTemplates CR, that are available for a user. The Module Catalog portfolio may vary for different users.
The synchronization mechanism described below is essential to allow users to enable modules in their clusters.
The mechanism is controlled by the set of labels that are configured on Kyma and ModuleTemplate CRs in the Control Plane. The labels are: `operator.kyma-project.io/sync`, `operator.kyma-project.io/internal`, and `operator.kyma-project.io/beta`.
The v1beta2 API introduces three groups of modules:

- Default modules, synchronized by default.
- Internal modules, synchronized per-cluster only if configured explicitly on the corresponding Kyma CRs. To mark a ModuleTemplate CR as `internal`, use the `operator.kyma-project.io/internal` label and set it to `true`.
- Beta modules, synchronized per-cluster only if configured explicitly on the corresponding Kyma CRs. To mark a ModuleTemplate CR as `beta`, use the `operator.kyma-project.io/beta` label and set it to `true`.

By default, without any labels configured on Kyma and ModuleTemplate CRs, a ModuleTemplate CR is synchronized with remote clusters.

**NOTE:** The ModuleTemplate CRs synchronization is enabled only when Lifecycle Manager runs in the [control-plane mode](../running-modes.md). Lifecycle Manager running in the single-cluster mode, doesn't require any CR synchronization.

**NOTE:** Disabling synchronization for already synchronized ModuleTemplates CRs doesn't remove them from remote clusters. The CRs remain as they are, but any subsequent changes to these ModuleTemplate CRs in the Control Plane are not synchronized.

For details, read about [the Kyma CR synchronization labels](kyma-cr.md#operatorkyma-projectio-labels) and [the ModuleTemplate CR synchronization labels](moduleTemplate-cr.md#operatorkyma-projectio-labels).

## Stability

See the list of CRs involved in Lifecycle Manager's workflow and their stability status:

| Version | Name                                                | Stability                                                                                                                                                                                                    |
|:--------|-----------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| v1beta2 | [Kyma](/api/v1beta2/kyma_types.go)                         | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [ModuleTemplate](/api/v1beta2/moduletemplate_types.go)     | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Manifest](/api/v1beta2/manifest_types.go)                 | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
| v1beta2 | [Watcher](/api/v1beta2/watcher_types.go)                   | Beta-Grade - no breaking changes without API incrementation. Use for automation and watch upstream as close as possible for deprecations or new versions. Alpha API is deprecated and converted via webhook. |
