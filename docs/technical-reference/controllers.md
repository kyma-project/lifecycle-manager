# Controllers

This document describes the controllers used by Lifecycle Manager.

## Kyma Controller

[Kyma Controller](../../internal/controller/kyma/controller.go) deals with the introspection, interpretation, and status update of the [Kyma custom resource (CR)](../../api/v1beta2/kyma_types.go).

Its main responsibilities are:

1. Interpret the **.spec.modules** list and use the correct [ModuleTemplate CR](../../api/v1beta2/moduletemplate_types.go) for a module.
2. Translate the ModuleTemplate CR into a [Manifest CR](../../api/v1beta2/manifest_types.go) and create it with an OwnerReference to the Kyma CR where the module was listed.
3. Propagate changes from ModuleTemplate CR updates (e.g. updates to the Module Layers contained in the OCI Descriptor) into the correct Manifest CR and process upgrades, but prohibit downgrades.
4. Track all created Manifest CRs and aggregate the status into a `State`, that reflects the integrity of the Kyma installation managed by Lifecycle Manager.
5. Synchronize all the above changes to the Kyma CR Status as well as available ModuleTemplate CRs into a remote cluster.
To determine the cluster to sync to, fields in **.spec.remote** are evaluated.
This allows the use of ModuleTemplate CRs in a cluster managed by Lifecycle Manager while Kyma Control Plane is in a different cluster.

### Remote Synchronization

The Kyma CR in Kyma Control Plane shows the initial specification and the current status. To install a module, Lifecycle Manager uses the specification from the remote cluster Kyma CR.

## Mandatory Modules Controllers

Lifecycle Manager uses two Mandatory Modules Controllers:

* [Mandatory modules installation controller](../../internal/controller/mandatorymodule/installation_controller.go) deals with the reconciliation of mandatory modules
* [Mandatory modules deletion controller](../../internal/controller/mandatorymodule/deletion_controller.go) deals with the deletion of mandatory modules

Since the channel concept does not apply to mandatory modules, the Mandatory Modules Installation Controller fetches all the Mandatory ModuleTemplate CRs without any channel filtering. It then translates the ModuleTemplate CR for the mandatory module to a [Manifest CR](../../api/v1beta2/manifest_types.go) with an OwnerReference to the Kyma CR. Similarly to the [Kyma Controller](../../internal/controller/kyma/controller.go),
it propagates changes from the ModuleTemplate CR to the Manifest CR. The mandatory ModuleTemplate CR is not synchronized to the remote cluster and the module status does not appear in the Kyma CR status. If a mandatory module needs to be removed from all clusters, the corresponding ModuleTemplate CR needs to be deleted. The Mandatory Module Deletion Controller picks this event up and marks all associated Manifest CRs for deletion. To ensure that the ModuleTemplate CR is not removed immediately, the controller adds a finalizer to the ModuleTemplate CR. Once all associated Manifest CRs are deleted, the finalizer is removed and the ModuleTemplate CR is deleted.

## Manifest Controller

[Manifest controller](../../internal/controller/manifest/controller.go) deals with the reconciliation and installation of data desired through a Manifest CR, a representation of a single module desired in a cluster.
Since it mainly is a delegation to the [declarative reconciliation library](../../internal/declarative/README.md) with certain [internal implementation additions](../../internal/manifest/README.md), please look at the respective documentation for these parts to understand them more.

## Purge Controller

[Purge controller](../../internal/controller/purge/controller.go) is responsible for handling the forced cleanup of deployed resources in a remote cluster when its Kyma CR is marked for deletion.
Suppose a Kyma CR has been marked for deletion for longer than the grace period (default is 5 minutes). In that case, the controller resolves the remote client for the cluster, retrieves all relevant CRs deployed on the cluster, and removes finalizers, allowing the resources to be garbage collected. This ensures that all associated resources are properly purged, maintaining the integrity and cleanliness of the cluster.

## Watcher Controller

[Watcher controller](../../internal/controller/watcher/controller.go) deals with the update of VirtualService rules derived from the [Watcher CR](../../api/v1beta2/watcher_types.go). This is then used to initialize the Watcher CR from the Kyma Controller in each runtime, a small component initialized to propagate changes from the runtime (remote) clusters back to react to changes that can affect the Manifest CR integrity.
