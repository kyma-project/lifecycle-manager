# Modularization

Modules are the next generation of components in Kyma that are available for local and cluster installation.

Modules are no longer represented by a single helm-chart, but instead are bundled and released within channels through a [ModuleTemplate custom resource (CR)](../api/v1beta2/moduletemplate_types.go), a unique link of a module, and its desired state of resources and configuration, and a channel.

Lifecycle Manager manages clusters using the [Kyma CR](../api/v1beta2/kyma_types.go). The CR defines the desired state of modules in a cluster. With the CR you can enable and disable modules with domain-specific functionality with additional configuration.

The modules themselves are built and distributed as OCI artifacts. The internal structure of the artifact conforms to the [Open Component Model](https://ocm.software/) scheme version 3. Modules contain an immutable layer set of a module operator deployment description and its configuration.

![Kyma Module Structure](/docs/assets/kyma-module-template-structure.svg)

If you use Kyma [CLI](https://github.com/kyma-project/cli), you can create a Kyma module by running `kyma alpha create module`. This command packages all the contents on the provided path as an OCI artifact and pushes the artifact to the provided OCI registry. Use the `kyma alpha create module --help` command to learn more about the module structure and how it is created. You can also use the CLI's auto-detection of [Kubebuilder](https://kubebuilder.io) projects to easily bundle your module with little effort.

The modules are installed and controlled by Lifecycle Manager. We use [Open Component Model](https://ocm.software) to describe all of our modules descriptively.
Based on the [ModuleTemplate CR](../api/v1beta2/moduletemplate_types.go), the module is resolved from its layers and version and is used as a template for the [Manifest CR](api/v1beta1/manifest_types.go).
Whenever a module is accepted by Lifecycle Manager the ModuleTemplate CR gets translated into a Manifest CR, which describes the actual desired state of the module operator.

The Lifecycle Manager then updates the [Kyma CR](../api/v1alpha2/kyma_types.go) of the cluster based on the observed status changes in the module CR (similar to a native Kubernetes deployment tracking availability).

Module operators only have to watch their custom resources and reconcile modules in the target clusters to the desired state.

## Example

A sample Kyma CR could look like this:

```bash
apiVersion: operator.kyma-project.io/v1beta1
kind: Kyma
metadata:
  name: my-kyma
spec:
  modules:
  - name: my-module
```

The creation of the Kyma CR triggers a reconciliation that:

1. Looks for a ModuleTemplate CR based on the search criteria, for example, the OCM Component Name of the module or simply the name of the ModuleTemplate CR.
2. Creates a Manifest CR for `my-module` based on a ModuleTemplate CR found in the cluster by resolving all relevant image layers for the installation.
3. Installs the content of the modules operator by applying it to the cluster, and observing its state.
4. Reports back all states observed in the Manifest CR which then get propagated to the Kyma CR on the cluster.
   Lifecycle Manager then uses the observed states to aggregate and combine the readiness condition of the cluster and determine the installation state or trigger more reconciliation loops as needed.

As mentioned above, when each module operator completes their installation, it reports its resource status. However, to accurately report the state, we read out the `.status.state` field to accumulate status reporting for the entire cluster.
