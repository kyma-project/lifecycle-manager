# Modularization

Lifecycle Manager manages clusters using the [Kyma](api/v1beta1/kyma_types.go) custom resource (CR). The CR defines the desired state of modules in a cluster. With the CR you can enable and disable modules with domain-specific functionality with additional configuration.

The modules themselves are bundled containers based on the [OCI Image Format Specification](https://github.com/opencontainers/image-spec). They contain an immutable layer set of a module operator deployment description and its configuration.

![Kyma Module Structure](/assets/kyma-module-template-structure.svg)

If you use Kyma's [CLI](https://github.com/kyma-project/cli), please refer to the `kyma alpha create module --help` section to learn more about module's structure and how it is created. You might even be able to use its inbuilt auto-detection of [kubebuilder](https://kubebuilder.io) projects to easily bundle your module with little effort.

The modules are installed and controlled by Lifecycle Manager. We use [Open Component Model](https://ocm.software) to describe all of our modules descriptively.
Based on the [ModuleTemplate Custom Resource](../api/v1beta1/moduletemplate_types.go), the module is resolved from its individual layers and version and is used as a template for the [Manifest](api/v1beta1/manifest_types.go).
Whenever a module is accepted by Lifecycle Manager the ModuleTemplate gets translated into a Manifest, which describes the actual desired state of the module operator.

The Lifecycle Manager then updates the [Kyma Custom Resource](../api/v1alpha1/kyma_types.go) of the cluster based on the observed status changes in the Module Custom Resources (similar to a native kubernetes deployment tracking availability).

Module operators only have to watch their own custom resources and reconcile modules in the target clusters to the desired state.

### Example

A sample `Kyma` CR could look like this:
```
apiVersion: operator.kyma-project.io/v1beta1
kind: Kyma
metadata:
  name: my-kyma
spec:
  modules:
  - name: my-module
```

The creation of the custom resource triggers a reconciliation that
1. looks for a ModuleTemplate based on search criteria, for example the OCM Component Name of the Module or simply the of the `ModuleTemplate`
2. creates a `Manifest` for `my-module` based on a [ModuleTemplate](api/v1beta1/moduletemplate_types.go) found in the cluster by resolving all relevant image layer for the installation
3. installing the contents of the modules operator by applying them to the cluster, and observing its state
4. reporting back all states observed in the `Manifest` which then gets propagated to the `Kyma` resource for the cluster.
   Lifecycle Manager then uses this to aggregate and combine the readiness condition of the cluster and determine the installation state or trigger more reconciliation loops as needed.

As mentioned above, when each module operator completes their installation, it reports its own resource status. However, to accurately report state, we read out the `.status.state` field to accumulate status reporting for an entire cluster.
