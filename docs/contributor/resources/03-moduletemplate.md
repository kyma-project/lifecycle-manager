# ModuleTemplate

The `moduletemplates.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to configure the ModuleTemplate resource.

The ModuleTemplate custom resource (CR) defines a module, in a particular version, that can be added to or deleted from the module list in the Kyma CR. Each ModuleTemplate CR represents one module.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd moduletemplates.operator.kyma-project.io -o yaml
```

> ### Note
> The ModuleTemplate CR is applied in both Kyma Control Plane (KCP) and SAP BTP, Kyma runtime clusters.
> Lifecycle Manager synchronizes the ModuleTemplates from KCP to the applicable Kyma runtime instances.
> See [Module Catalog Synchronization](../08-kcp-skr-synchronization.md#module-catalog-synchronization) for more details.

## Configuration

### **.spec.channel (Deprecated)**

The `channel` field previously indicated the channel in which a ModuleTemplate CR was registered. It was used alongside the channel attributes of the Kyma CR to match a module with a specific channel.

**Note:** This field is now deprecated and will be removed in a future release. It has been decided that ModuleTemplates are now tied directly to versions, rather than being associated with channels.

For the following ModuleTemplate CR:

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleTemplate
metadata:
  name: moduletemplate-sample
spec:
  channel: regular
```

The module was referenced by a Kyma CR asking for it in the `regular` channel.

### **.spec.data**

The data that should be used for the initialization of a custom resource after the module has been installed. It is only used if the `.spec.modules[].customResourcePolicy` in the Kyma CR is set to `CreateAndDelete`. The data field must be filled with a valid custom resource (that can be of any type available in the API-Server _after_  module initialization). If set to `Ignore` by the module specification of the Kyma CR, it is entirely ignored, even when filled.

A (partial) example could look like this:

```yaml
spec:
  data:
    apiVersion: operator.kyma-project.io/v1alpha1
    kind: Keda
    metadata:
      name: keda-sample
    spec:
      resources:
        operator:
          limits:
            cpu: "800m"
            memory: "200Mi"
          requests:
            cpu: "300m"
            memory: "150Mi"
```

If not specified, the **namespace** of the resource mentioned in **.spec.data** will be controlled by the `sync-namespace` flag; otherwise, it will be respected. All other attributes (including **.metadata.name**, **apiVersion**, and **kind**) are taken over as stated. Note that since it behaves similarly to a `template`, any subresources, such as **status**, are ignored, even if specified in the field.

### **.spec.info**

The **info** field contains module metadata, including the repository URL, documentation link, and icons. For example:

```yaml
spec:
  info:
    repository: https://github.com/example/repo
    documentation: https://docs.example.com
    icons:
    - name: example-icon
      link: https://example.com/icon.png
```

- repository: The link to the repository of the module.
- documentation: The link to the documentation of the module.
- icons: A list of icons of the module, each with a name and link.

### **.spec.manager**

The `manager` field provides information for identifying a module's resource that can indicate the module's installation readiness. Typically, the manager is the module's `Deployment` resource. In exceptional cases, it may also be another resource. The **namespace** parameter is optional if the resource is not namespaced. For example,  if the resource is the module CR's `CustomResourceDefinition`.

In this example, the module's manager is the `Deployment` resource in the `kyma-system` namespace.

```yaml
spec:
  manager:
    group: apps
    version: v1
    kind: Deployment
    namespace: kyma-system
    name: [module manager name]
```

In this example, the module's manager is the module's `CustomResourceDefinition` that does not require the **namespace** parameter.

```yaml
spec:
  manager:
    group: apiextensions.k8s.io
    version: v1
    kind: CustomResourceDefinition
    name: [module CRD name]
```
### **.spec.customStateCheck (Deprecated)**

> ### Warning
> This field was deprecated at the end of July 2024 and will be deleted in the next ModuleTemplate API version. As of the deletion day, you can define the custom state only in a module's custom resource.

The `.spec.customStateCheck` field in Kyma Lifecycle Manager is primarily designed for third-party modules. For non-Kyma modules, the `status.state` might not be present, which the Lifecycle Manager relies on to determine the module state. This field enables users to define custom fields in the module Custom Resource (CR) that can be mapped to valid states supported by Lifecycle Manager.

Imagine a scenario where a module's health is indicated by `status.health` in its CR. In such cases, users can employ the customStateCheck configuration to map the health states to Lifecycle Manager states.

Here's an example of YAML configuration:

```yaml
spec:
  customStateCheck:
  - jsonPath: 'status.health'
    value: 'green'
    mappedState: 'Ready'
  - jsonPath: 'status.health'
    value: 'red'
    mappedState: 'Error'
```

In this example, when the module's CR is in the green health state, the corresponding Kyma CR will transition to the `Ready` state. Similarly, when the module's CR is in the red health state, the related Kyma CR will transition to the `Error` state.

The valid mappedState values are defined in the Kyma CR API.

Furthermore, this field supports complex mappings. For instance, if multiple states are needed to determine the `Ready` state, users can define the following customStateCheck:

```yaml
spec:
  customStateCheck:
  - jsonPath: 'module.state.field1'
    value: 'value1'
    mappedState: 'Ready'
  - jsonPath: 'module.state.field2'
    value: 'value2'
    mappedState: 'Ready'
```

In this scenario, the `Ready` state will only be reached if both `module.state.field1` and `module.state.field2` have the respective specified values.

### **.spec.descriptor**

The core of any ModuleTemplate CR, the descriptor can be one of the schemas mentioned in the latest version of the [OCM Model Specification](https://github.com/open-component-model/ocm-spec/blob/7bfbc171e814e73d6e95cfa07cc85813f89a1d44/doc/01-model/01-model.md#components-and-component-versions). While it is a `runtime.RawExtension` in the Go types, it will be resolved via ValidatingWebhook into an internal descriptor with the help of the official [OCM library](https://github.com/open-component-model/ocm).

For more information on how to create ModuleTemplates with component descriptors using modulectl and OCM CLI, see [Creating ModuleTemplates](../14-creating-moduletemplate.md).

### **.spec.mandatory**

The `mandatory` field indicates whether the module is installed in all runtime clusters without any interaction from the user.
Mandatory modules do not appear in the Kyma CR `.status` and `.spec.modules`, furthermore they have the same configuration across all runtime clusters.

### **.spec.moduleName**

The name of the module. Used to refer to this module when adding the module to the **.spec.modules** list of the [Kyma CR](./01-kyma.md).

### **.spec.resources**

The `resources` field is a list of additional resources of the module that can be fetched. As of now, the primary expected use case is for module teams to add a link to the raw manifest of the module.

### **.spec.associatedResources**

The `associatedResources` field is a list of module-related custom resource definitions (CRDs) that should be cleaned up during module deletion.
The list is purely informational and does not introduce functional changes to the module.

### **.spec.requiresDowntime**

The `requiresDowntime` field indicates whether the module requires downtime to support maintenance windows during module upgrades. It is optional and defaults to `false`, meaning the module version upgrades don't require downtime.

## `operator.kyma-project.io` Labels

* `operator.kyma-project.io/mandatory-module`: A boolean value. Indicates whether the module is mandatory and must be installed in all remote clusters.
* `operator.kyma-project.io/internal`: A boolean value. If set to `true`, the ModuleTemplate CRs labeled with the same label, so-called `internal` modules, are also synchronized with the remote cluster. The default value is `false`.
* `operator.kyma-project.io/beta`: A boolean value. If set to `true`, the ModuleTemplate CRs labeled with the same label, so-called `beta` modules, are also synchronized with the remote cluster. The default value is `false`.

## `operator.kyma-project.io` Annotation

* `operator.kyma-project.io/is-cluster-scoped`: A boolean value. Indicates whether the module configured is a namespace-scoped or cluster-scoped resource.
