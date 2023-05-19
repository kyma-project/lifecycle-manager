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
            memory: "200Mi"
          requests:
            cpu: "300m"
            memory: "150Mi"
```

If not specified, the **namespace** of the resource mentioned in **.spec.data** will be controlled by the `sync-namespace` flag; otherwise, it will be respected. All other attributes (including **.metadata.name**, **apiVersion**, and **kind**) are taken over as stated. Note that since it behaves similarly to a `template`, any subresources, such as **status**, are ignored, even if specified in the field.

### **.spec.descriptor**

The core of any ModuleTemplate CR, the descriptor can be one of the schemas mentioned in the latest version of the [OCM Software Specification](https://ocm.software/spec/). While it is a `runtime.RawExtension` in the Go types, it will be resolved via ValidatingWebhook into an internal descriptor with the help of the official [OCM library](https://github.com/open-component-model/ocm).

By default, it will most likely be easiest to use [Kyma CLI](https://github.com/kyma-project/cli/tree/main) and its `create module` command to create a template with a valid descriptor, but it can also be generated manually, for example using [OCM CLI](https://github.com/open-component-model/ocm/tree/main/cmds/ocm).

### `operator.kyma-project.io` labels

These are the synchronization labels available on the ModuleTemplate CR:

- `operator.kyma-project.io/sync`: A boolean value. If set to `false`, this ModuleTemplate CR is not synchronized with any remote cluster. The default value is `true`.
- `operator.kyma-project.io/internal`: A boolean value. If set to `true`, marks the ModuleTemplate CR as an `internal` module. It is then synchronized only for these remote clusters which are managed by the Kyma CR with the same `operator.kyma-project.io/internal` label explicitly set to `true`. The default value is `false`.
- `operator.kyma-project.io/beta` A boolean value. If set to `true`, marks the ModuleTemplate CR as a `beta` module. It is then synchronized only for these remote clusters which are managed by the Kyma CR with the same `operator.kyma-project.io/beta` label explicitly set to `true`. The default value is `false`.
