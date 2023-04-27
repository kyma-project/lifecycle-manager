# Lifecycle Manager API

The Lifecycle Manager API types considers three major pillars that each deal with a specific aspect of reconciling modules into their corresponding states.

1. The introduction of a single entry point CustomResourceDefinition to control a Cluster and it's desired state: The [`Kyma` CustomResource](v1beta1/kyma_types.go)
2. The introduction of a single entry point CustomResourceDefinition to control a Module and it's desired state: The [`Manifest` CustomResource](v1beta1/manifest_types.go)
3. The [`ModuleTemplate` CustomResource](v1beta1/moduletemplate_types.go) which contains all reference data for the modules to be installed correctly. It is a standardized desired state for a module available in a given release channel.

Additionally, we maintain the [`Watcher` CustomResource](v1beta1/watcher_types.go) to define callback functionality for synchronized remote clusters that allows lower latencies before changes are detected by the control-plane.
