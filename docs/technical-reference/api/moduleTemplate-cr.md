# ModuleTemplate Custom Resource

The core of our modular discovery, the [ModuleTemplate](../../../api/v1beta1/moduletemplate_types.go) contains 3 main parts that are used to initialize and resolve modules.

### `.spec.channel`

The channel that the ModuleTemplate is registered in. It is used alongside the channel attributes of the `Kyma` to match up a module and a channel.

For a ModuleTemplate

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: ModuleTemplate
metadata:
  name: moduletemplate-sample
spec:
  channel: regular
```

the module will be referenced by any Kyma asking for it in the `regular` channel.

### `.spec.data`

The data that should be used for initialization of a CustomResource after the module has been installed. It is only used if the `customResourcePolicy` is set to `CreateAndDelete` and it is filled with a valid CustomResource (that can be of any type available in the API-Server _after_  module initialization). If set to `Ignore` by the module specification of `Kyma` it is entirely ignored, even when filled.

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

The `namespace` of the resource mentioned in `.spec.data` will be defaulted to `.spec.sync.namespace` or `.metadata.namespace` of the referencing `Kyma` if not filled, otherwise it will be respected. All other attributes (including `.metadata.name` and `apiVersion` and `kind`) are taken over as is. Note that since it behaves similarly to a `template` any subresources, like `status` are ignored, even if specified in the field.

### `.spec.descriptor`

The core of any ModuleTemplate, the descriptor can be one of the schemas mentioned in the latest version of the [OCM Software Specification](https://ocm.software/spec/). While it is a `runtime.RawExtension` in the Go types, it will be resolved via ValidatingWebhook into an internal descriptor with the help of the official [OCM library](https://github.com/open-component-model/ocm).

By default, it will most likely be easiest to use [Kyma CLI](https://github.com/kyma-project/cli/tree/main) and its `create module` command to create a template with a valid descriptor, but it can also be generated manually, e.g. with [OCM CLI](https://github.com/open-component-model/ocm/tree/main/cmds/ocm)

### `.spec.target`

The `target` attribute may be simple, but it can have a great effect on what is happening on module initialization. It can be one of `control-plane` and `remote`. While `remote` will simply use the connection established by `.spec.sync.strategy` of the referencing `Kyma`, the `control-plane` mode will cause the module to be initialized inside the control-plane cluster for every instance of the module in the runtime.

The `control-plane` mode should be used when:
1. Developing on a single cluster, with a monolithic architecture instead of split runtime and control-plane, in which case a singleton module can be initialized as if it were runniing in the remote.
2. Developing a module which has resource requirements or third-party prerequisites that require it to use parts of the control-plane (e.g. an independent connection to the cluster or auditing work)

For the most part, it should be noted that the [Kyma CLI](https://github.com/kyma-project/cli/tree/main) will create modules with the `control-plane` mode to streamline local development.