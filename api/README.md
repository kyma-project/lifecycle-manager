# Lifecycle Manager API

The Lifecycle Manager API types considers three major pillars that each deal with a specific aspect of reconciling Modules into their corresponding states.

1. The introduction of a single entry point CustomResourceDefinition to control a Cluster and it's desired state: The [`Kyma` CustomResource](v1beta1/kyma_types.go)
2. The introduction of a single entry point CustomResourceDefinition to control a Module and it's desired state: The [`Manifest` CustomResource](v1beta1/manifest_types.go)
3. The [`ModuleTemplate` CustomResource](v1beta1/moduletemplate_types.go) which contains all reference data for the modules to be installed correctly. It is a standardized desired state for a module available in a given release channel.

## Kyma Custom Resource

The [`Kyma` CustomResource](v1beta1/kyma_types.go) contains 3 fields that are together used to declare the desired state of a cluster:

1. `.spec.channel`: The Release Channel that should be used by default for all modules that are to be installed in the cluster.
2. `.spec.modules`: The Modules that should be installed into the cluster. Each Module contains a name (which we will get to later) serving as a link to a `ModuleTemplate`. Additionally one can add a specific channel (if `.spec.channel`) should not be used. On Top of that one can specify a `controller`, which serves as a Multi-Tenant Enabler. It can be used to only listen to ModuleTemplates provided under the same controller-name. Last but not least, it includes a `customResourcePolicy` which can be used for specifying defaulting behavior when initialising modules in a cluster.
3. `.spec.sync`: Various settings to enable synchronization of the `Kyma` and `ModuleTemplate` CustomResources into a remote cluster that is separate from the control-plane (the cluster where Lifecycle Manager is deployed).

## ModuleTemplate Custom Resource

## Manifest Custom Resource

## Watcher Custom Resource
