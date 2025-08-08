# Lifecycle Manager Controllers

This document describes the controllers used by Lifecycle Manager.

## Kyma Controller

[Kyma Controller](https://github.com/kyma-project/lifecycle-manager/blob/main/internal/controller/kyma/controller.go) deals with the introspection, interpretation, and status update of the [Kyma custom resource (CR)](./resources/01-kyma.md).

Its main responsibilities are:

1. Interpret the **.spec.modules** list and use the correct [ModuleTemplate CR](./resources/03-moduletemplate.md) for a module.
2. Translate the ModuleTemplate CR into a [Manifest CR](./resources/02-manifest.md) and create it with an OwnerReference to the Kyma CR where the module was listed.
3. Propagate changes from ModuleTemplate CR updates (e.g. updates to the Module Layers contained in the OCI Descriptor) into the correct Manifest CR and process upgrades, but prohibit downgrades.
4. Track all created Manifest CRs and aggregate the status into a `State`, that reflects the integrity of the Kyma installation managed by Lifecycle Manager.
5. Synchronize all the above changes to the Kyma CR Status as well as available ModuleTemplate CRs into a remote cluster.
To determine the cluster to sync to, fields in **.spec.remote** are evaluated.
This allows the use of ModuleTemplate CRs in a cluster managed by Lifecycle Manager while Kyma Control Plane is in a different cluster.

### Fetching the ModuleTemplate

Kyma Controller uses the ModuleReleaseMeta CR to fetch the correct ModuleTemplate CR for a module. The name of ModuleReleaseMeta CR should be the same as the module name. Kyma Controller uses the channel defined in the Kyma CR spec to fetch the corresponding module version from the ModuleReleaseMeta channel-version pairs. Kyma Controller then fetches the ModuleTemplate CR with the module name-version from the ModuleTemplate CRs available in the Kyma Control Plane. If there is no entry in the ModuleReleaseMeta CR for the channel defined in the Kyma CR spec, the Kyma CR will be in the `Error` state indicating that no versions were found for the channel.

If a ModuleReleaseMeta CR for a particular module doesn't exist, Kyma Controller lists all the ModuleTemplates in the Control Plane and then filters them using the **.spec.channel** parameter in the Kyma CR.

### Requeuing the Kyma CR

The `Kyma` CR is requeued at set intervals using specific flags from Lifecycle Manager. The requeuing ensures that the Kyma CR is periodically reprocessed, allowing the controller to detect and apply any changes that may have occurred during that time. Additionally, several watch mechanisms are implemented, enabling the controller to requeue Kyma CRs when certain events occur.

These watch mechanisms monitor Kyma, Secret, Manifest, and ModuleReleaseMeta CRs, ensuring that the relevant Kyma CRs are requeued whenever these CRs are created, updated, or deleted. Additionally, the watch mechanism for ModuleReleaseMeta CRs has a dedicated implementation in "ModuleReleaseMetaEventHandler", which ensures that all Kyma CRs using a module in a channel affected by the ModuleReleaseMeta CR are requeued as needed.

## Mandatory Modules Controllers

Lifecycle Manager uses two Mandatory Modules Controllers:

* Mandatory modules installation controller deals with the reconciliation of mandatory modules
* Mandatory modules deletion controller deals with the deletion of mandatory modules

Since the channel concept does not apply to mandatory modules, the Mandatory Modules Installation Controller fetches all the Mandatory ModuleTemplate CRs with the 'operator.kyma-project.io/mandatory-module' label. If multiple ModuleTemplates exist for the same mandatory module, the Controller fetches the ModuleTemplate with the highest version. It then translates the ModuleTemplate CR for the mandatory module to a [Manifest CR](./resources/02-manifest.md) with an OwnerReference to the Kyma CR. Similarly to the Kyma Controller,
it propagates changes from the ModuleTemplate CR to the Manifest CR. The mandatory ModuleTemplate CR is not synchronized to the remote cluster and the module status does not appear in the Kyma CR status. If a mandatory module needs to be removed from all clusters, the corresponding ModuleTemplate CR needs to be deleted. The Mandatory Module Deletion Controller picks this event up and marks all associated Manifest CRs for deletion. To ensure that the ModuleTemplate CR is not removed immediately, the controller adds a finalizer to the ModuleTemplate CR. Once all associated Manifest CRs are deleted, the finalizer is removed and the ModuleTemplate CR is deleted.

## Manifest Controller

Manifest controller deals with the reconciliation and installation of data desired through a Manifest CR, a representation of a single module desired in a cluster.
Since it mainly is a delegation to the declarative reconciliation library with certain internal implementation additions, please look at the respective documentation for these parts to understand them more.

## Purge Controller

Purge controller is responsible for handling the forced cleanup of deployed resources in a remote cluster when its Kyma CR is marked for deletion.
Suppose a Kyma CR has been marked for deletion for longer than the grace period (default is 5 minutes). In that case, the controller resolves the remote client for the cluster, retrieves all relevant CRs deployed on the cluster, and removes finalizers, allowing the resources to be garbage collected. This ensures that all associated resources are properly purged, maintaining the integrity and cleanliness of the cluster.

## Watcher Controller

Watcher controller deals with the changes of VirtualService rules derived from the [Watcher CR](./resources/04-watcher.md). This is then used to initialize the Watcher CR from the Kyma Controller in each runtime. Simply put, it is a small component initialized to propagate changes from the runtime (remote) clusters back to the Kyma Control Plane (KCP), for it to react to the changes accordingly, ensuring the integrity of the affected Manifest CRs.

## Istio Gateway Secret Controller

Istio Gateway Secret controller manages the certificate secret used by the Istio gateway. Its main responsibility is to bundle previous and new self-signed watcher CA certificates during rotation, so that Kyma runtimes, whose certificates have not been signed by the new CA certificate, can also authenticate with the gateway. This ensures zero downtime of the watch mechanism.
