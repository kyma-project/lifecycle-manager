# API Changelog

This document represents changes introduced by an API version.

## v1beta1 to v1beta2

The v1beta1 to v1beta2 version change introduced the following changes to Lifecycle Manager's custom resoucres (CRs).

### Kyma CR

The main change introduced to the Kyma CRD is the removal of the  **.spec.sync** attribute. As a result, the v1beta1 **.spec.sync** sub-attributes handling changes as described:

- **.sync.enabled** - replaced by the `operator.kyma-project.io/sync` label in a Kyma CR. For details, read the [Kyma CR synchronization labels](link TBD) document.
- **.sync.moduleCatalog** - replaced by a combination the `operator.kyma-project.io/sync`, `operator.kyma-project.io/internal`, and `operator.kyma-project.io/beta` labels in Kyma and ModuleTemplate CRs. For details, read the [Kyma CR synchronization labels](link TBD) document.
- **.sync.strategy** - replaced with **sync-strategy** annotation in a Kyma CR. By default, the value for the **sync.strategy** annotation is `local-secret`, other values are used for testing purposes only.
- **.sync.namespace** - replaced with a `sync-namespace` command-line flag for Lifecycle Manager. It means that a user can no longer configure the Namespace synchronized with a particular Kyma CR. The Namespace is the same for all Kyma CRs in a given Lifecycle Manager instance, and a user can't change it.
- **.sync.noModuleCopy** - removed. Currently the **.spec.modules[]** for a remote Kyma CR is always initialized as empty.

### ModuleTemplate CR

In the ModuleTemplate CRD the changes relate to the **sync.target** attribute:

- `.sync.target` - replaced with a `in-kcp-mode` command-line flag for Lifecycle Manager. It means that a user can no longer configure the ModuleTemplate synchronization. The configuration is the same for all ModuleTemplate CRs in a given Lifecycle Manager instance, and a user can't change it.
