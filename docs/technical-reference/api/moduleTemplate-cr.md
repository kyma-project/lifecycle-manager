# ModuleTemplate Custom Resource

The core of our modular discovery, the [ModuleTemplate custom resource (CR)](../../../api/v1beta2/moduletemplate_types.go) contains 3 main parts that are used to initialize and resolve modules.

### **.spec.channel**

The channel that a ModuleTemplate CR is registered in. It is used alongside the channel attributes of the Kyma CR to match up a module and a channel.

For the following ModuleTemplate CR:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: ModuleTemplate
metadata:
  name: moduletemplate-sample
spec:
  channel: regular
```

the module will be referenced by any Kyma CR asking for it in the `regular` channel.

### **.spec.data**

The data that should be used for the initialization of a custom resource after the module has been installed. It is only used if the `customResourcePolicy` is set to `CreateAndDelete` and it is filled with a valid custom resource (that can be of any type available in the API-Server _after_  module initialization). If set to `Ignore` by the module specification of the Kyma CR, it is entirely ignored, even when filled.

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

### **.spec.customStateCheck**

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

In this example, when the module's CR is in the green health state, the corresponding Kyma CR will transition to the Ready state. Similarly, when the module's CR is in the red health state, the related Kyma CR will transition to the Error state.

The valid mappedState values are defined in the [Kyma CR API](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/kyma_types.go#L225-L245).

Furthermore, this field supports complex mappings. For instance, if multiple states are needed to determine the Ready state, users can define the following customStateCheck:
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

In this scenario, the Ready state will only be reached if both module.state.field1 and module.state.field2 have the respective specified values.

### **.spec.descriptor**

The core of any ModuleTemplate CR, the descriptor can be one of the schemas mentioned in the latest version of the [OCM Software Specification](https://ocm.software/spec/). While it is a `runtime.RawExtension` in the Go types, it will be resolved via ValidatingWebhook into an internal descriptor with the help of the official [OCM library](https://github.com/open-component-model/ocm).

By default, it will most likely be easiest to use [Kyma CLI](https://github.com/kyma-project/cli/tree/main) and its `create module` command to create a template with a valid descriptor, but it can also be generated manually, for example using [OCM CLI](https://github.com/open-component-model/ocm/tree/main/cmds/ocm).

### `operator.kyma-project.io` labels

These are the synchronization labels available on the ModuleTemplate CR:

- `operator.kyma-project.io/sync`: A boolean value. If set to `false`, this ModuleTemplate CR is not synchronized with any remote cluster. The default value is `true`.
- `operator.kyma-project.io/internal`: A boolean value. If set to `true`, marks the ModuleTemplate CR as an `internal` module. It is then synchronized only for these remote clusters which are managed by the Kyma CR with the same `operator.kyma-project.io/internal` label explicitly set to `true`. The default value is `false`.
- `operator.kyma-project.io/beta` A boolean value. If set to `true`, marks the ModuleTemplate CR as a `beta` module. It is then synchronized only for these remote clusters which are managed by the Kyma CR with the same `operator.kyma-project.io/beta` label explicitly set to `true`. The default value is `false`.
