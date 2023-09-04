# Manage module enablement with CustomResourcePolicy

## Context

During the Module CR enablement, the default behavior of Lifecycle Manager is to:

1. Apply the configuration from the ModuleTemplate CR to the module CR,
2. Reset any direct changes to the module CR during reconciliation.

This can be inconvenient for some use cases that require more flexibility and external control over the module CR enablement.

To address this issue, we propose a CustomResourcePolicy feature that allows users to specify how Lifecycle Manager should handle the configuration of the module CR during enablement and reconciliation.

## Procedure

With Kyma CLI, enable a module with the [`kyma alpha enable`](https://github.com/kyma-project/cli/blob/main/docs/gen-docs/kyma_alpha_enable.md) command. Using the CLI, you can manage the CustomResourcePolicy for each module individually.

By default, the CustomResourcePolicy of the enabled module is `CreateAndDelete`. With the default, you let the Lifecycle Manager take full control over the module enablement.

For example, to enable the Keda module with the default policy for the `default-kyma` Kyma CR, run:

```bash
kyma alpha enable module keda -n kyma-system -k default-kyma
```

This will result in the `default-kyma` Kyma CR spec like this:

```bash
spec:
  channel: alpha
  modules:
  - customResourcePolicy: CreateAndDelete
    name: keda
```

Lifecycle Manager will create a corresponding Keda CR in your target cluster and propagate all the values from the ModuleTemplate `spec.data.spec` to the `spec.resource` of the related Manifest CR. This way, you can configure and manage your Keda resources using Lifecycle Manager.

To skip this enablement process, you can set the `customResourcePolicy` flag to `Ignore` when you enable the module. This will result in no Keda CR created in your target cluster. It will also prevent Lifecycle Manager from adding any `spec.resource` to the related Manifest CR.

> **CAUTION:** Setting up the flag to 'Ignore' also means that Lifecycle Manager will not monitor or manage any Keda CR's readiness status. Therefore, you should exercise caution and discretion when using the `Ignore` policy for your module CR.
