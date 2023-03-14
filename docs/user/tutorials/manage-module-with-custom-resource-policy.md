# Managing module initialization with the CustomResourcePolicy

During the Module CR initialization, the default behavior of Lifecycle Manager is to apply the configuration from the Module Template to the Module CR, and to reset any direct changes to the Module CR during reconciliation. 

This can be inconvenient for some use cases that require more flexibility and external control over the Module CR initialization.

To address this issue, we propose a CustomResourcePolicy feature that allows users to specify how Lifecycle Manager should handle the configuration of the Module CR during initialization and reconciliation.

## Using Kyma CLI

With the Kyma CLI [enable module](https://github.com/kyma-project/cli/blob/main/docs/gen-docs/kyma_alpha_enable.md) command, you can manage the CustomResourcePolicy for each Module individually.

By default, the CustomResourcePolicy of enabled Module will always be `CreateAndDelete`. 
With this configuration, the Lifecycle Manager take fully control on Module initialization.

For example, to enable the Keda Module with the default policy for `default-kyma` Kyma CR, users can run:
```
kyma alpha enable module keda -n kyma-system -k default-kyma
```

This will result in the `default-kyma` Kyma CR spec like this:
```
spec:
  channel: alpha
  modules:
  - customResourcePolicy: CreateAndDelete
    name: keda
```

Lifecycle Manager will create a corresponding Keda CR in your target cluster and propagate all the values from Module Template `spec.data.spec` to the `spec.resource` of the related Manifest CR. This way, you can configure and manage your Keda resources using Lifecycle Manager.

To skip this initialization process, you can set the `policy` flag to `Ignore` when you enable the module.

With this flag, the `modules.customResourcePolicy` will initialized with `Ignore`. You will observe there will be no Keda CR created in target cluster, this will also prevent Lifecycle Manager from adding any `spec.resource` to the related Manifest CR. 