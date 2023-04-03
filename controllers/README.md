# Controllers used within Lifecycle-Manager

This package contains all controllers that can be registered within the Lifecycle Manager.
For more information on how the API behaves after the controller finishes up the synchronization, please look at the [API reference documentation](../api/README.md).

## Kyma Controller

The [Kyma controller](kyma_controller.go) is dealing with the introspection, interpretation and status update of the [`Kyma` CustomResource](../api/v1beta1/kyma_types.go).

Its main responsibilities are:

1. Interpret the `.spec.modules` list and use the correct [`ModuleTemplate` CustomResource](../api/v1beta1/moduletemplate_types.go) for the Module
2. Translate the `ModuleTemplate` into a [`Manifest` CustomResource](../api/v1beta1/manifest_types.go) and create it with an OwnerReference to the `Kyma` where the module was listed
3. Propagate changes from `ModuleTemplate` updates (e.g. updates to the Module Layers contained in the OCI Descriptor) into the correct `Manifest` and process upgrades, but prohibit downgrades.
4. Track all created `Manifests` and aggregate the status into a `State`, that reflects the integrity of the Kyma installation managed by Lifecycle Manager.
5. Synchronize all the above changes to the `Kyma` Status as well as available `ModuleTemplates` into a remote cluster.
   To determine the cluster to sync to, fields in `.spec.remote` are evaluated.
   This allows the use of ModuleTemplates in a cluster managed by the Lifecycle Manager
   while the control plane is in a different Cluster.

### Remote Synchronization

In order to synchronize remote clusters, the Kyma controller uses the concept of a _virtual_ resource.
The virtual resource is a superset of the specification of the control plane and runtime data of a module.
The synchronization of these is kept up-to-date with every reconciliation,
and will only be triggered if `.spec.sync.enabled` is set.
In this case, a so called `SyncContext` is initialized.
Every time the Kyma on the control plane is enqueued for synchronization,
it's spec is merged with the remote specification through our [custom synchronization handlers](../pkg/remote/kyma_synchronization_context.go).
These are not only able to synchronize the Kyma resource in the remote,
but they also replace the specification for all further parts of the reconciliation as a _virtual_ Kyma.
For more information, checkout the `ReplaceWithVirtualKyma` function.

On top of this, based on the  `.spec.sync.moduleCatalog` flag, the `syncModuleCatalog` is executed, which triggers a [reconciliation of all ModuleTemplates for discovery purposes]([custom synchronization handlers](../pkg/remote/remote_catalog.go).

## Manifest Controller

The [Manifest controller](manifest_controller.go) is dealing with the reconciliation and installation of data desired through a `Manifest`, a representation of a single module desired in a cluster.
Since it mainly is a delegation to the [declarative reconciliation library](../internal/declarative/README.md) with certain [internal implementation additions](../internal/manifest/README.md) please look at the respective documentation for these parts to understand them more.

## Watcher Controller

The [Watcher controller](watcher_controller.go) is dealing with the update of VirtualService rules derived from the [`Watcher` CustomResource](../api/v1beta1/watcher_types.go). This is then used to initialize the `Watcher` from the Kyma controller in each runtime, a small component initialized to propagate changes from the runtime(remote) clusters back to react to changes that can affect the `Manifest` integrity.
