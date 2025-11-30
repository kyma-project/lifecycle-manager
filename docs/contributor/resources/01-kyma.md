# Kyma

The `kymas.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to manage a cluster and its desired state. It contains the list of added modules and their state.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd kymas.operator.kyma-project.io -o yaml
```

The Kyma custom resource (CR) is used to declare the desired state of a cluster. **.spec.channel**, **.spec.modules[].channel**, and **.spec.modules** are the basic fields that are used together to define the cluster state.

* **.spec.channel** - defines a release channel that should be used by default for all modules that are to be installed in the cluster.
* **.spec.modules[].channel** - defines a release channel other than the default channel (**.spec.channel**) for a given module that is to be installed in the cluster.
* **.spec.modules** - specifies modules that should be added to the cluster. Each module contains a name serving as a link to the ModuleTemplate CR.
Additionally, you can add a specific channel if **.spec.channel** should not be used.
Last but not least, it includes a **customResourcePolicy** which can be used for specifying default behavior when initializing modules in a cluster.

> ### Note
> The Kyma CR is applied in both Kyma Control Plane (KCP) and SAP BTP, Kyma runtime clusters.
> Lifecycle-Manager synchronizes the `.state` from KCP to Kyma runtime.
> The `.spec` is only synchronized when creating the Kyma runtime resource from the KCP one.
> From then on, it is NOT synchronized any longer, and Lifecycle-Manager directly determines the desired state from the Kyma runtime resource.
> See [Kyma CR Synchronization](../08-kcp-skr-synchronization.md#kyma-cr-synchronization) for more details.

## Configuration

### **.spec.skipMaintenanceWindows**

Use the **skipMaintenanceWindows** parameter to indicate whether the module upgrades that require downtime should bypass the defined Maintenance Windows. If it is set to `true`, the module upgrade will happen as soon as a new module version is released in the Kyma Control Plane. 

### **.spec.channel** and **.spec.modules[].channel**

The **.spec.channel** attribute is used in conjunction with the [release channels](https://github.com/kyma-project/community/tree/main/concepts/modularization#release-channels). The channel that is used for the Kyma CR will always be used as the default in case no other specific channel is used.

This means that a Kyma CR that has the following spec:

```yaml
spec:
  channel: regular
  modules:
  - name: keda
  - name: serverless
```

attempts to look up the Keda and Serverless modules in the `regular` release channel.

However, if you specify channels using the **.spec.modules[].channel** attribute, the latter one is used instead.

```yaml
spec:
  channel: regular
  modules:
  - name: keda
    channel: fast
  - name: serverless
```

In this case, `fast` is the relevant channel for Keda, but not for Serverless.

### **.spec.modules**

The module list defines the desired set of all modules to be added to the Kyma runtime instance. A module must be added using its name. The module's name is defined as **.spec.moduleName** in both the ModuleReleaseMeta and the ModuleTemplate CRs.

Let's take a look at this simplified example:

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleTemplate
metadata:
  name: example-module-1.0.0
spec:
  moduleName: example-module
  version: 1.0.0
  data: {}
  descriptor:
    component:
      name: kyma-project.io/module/example-module
```

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleReleaseMeta
metadata:
  name: example-module
spec:
  channels:
  - channel: regular
    version: 1.0.0
  moduleName: example-module

```

The module mentioned above can be enabled by adding the `example-module` name to the **spec.modules.name** field of Kyma CR:
```yaml
spec:
  channel: regular
  modules:
  - name: example-module
```

> ### Warining
> Module referencing using NamespacedName and FQDN (Fully Qualified Domain Name) has been deprecated.

### **.spec.modules[].managed**

The **managed** field determines whether or not Lifecycle Manager manages a module. By default, the field is set to `true`. If you set it to `false`, you exclude a module from management by Lifecycle Manager.

For more information on how to unmanage a module, see [Setting Your Module to the Unmanaged and Managed State](../../user/02-unmanaging-modules.md).

### **.spec.modules[].customResourcePolicy**

In addition to this very flexible way of referencing modules, there is also another flag that can be important for users requiring more flexibility during module initialization. The `customResourcePolicy` flag is used to define one of `CreateAndDelete` and `Ignore`.
While `CreateAndDelete` causes the ModuleTemplate CR's **.spec.data** to be created and deleted to initialize a module with preconfigured defaults, `Ignore` can be used to only initialize the operator without initializing any default data.
This allows users to be fully flexible in regard to when and how to initialize their module.

### **.status.state**

The **state** attribute is a simple representation of the state of the entire Kyma CR installation. It is defined as an aggregated status that is either `Ready`, `Processing`, `Warning`, `Error`, or `Deleting`, based on the status of all Manifest CRs on top of the validity/integrity of the synchronization to a remote cluster if enabled.

- `Ready`: Indicates that the Kyma installation is ready. This means that the Kyma CR and module catalog are synced, all modules (Manifest CRs) are ready, and Watcher is installed.
- `Processing`: Indicates that the Kyma installation is processing. During this state, one of the following actions is being processed: the installation or uninstallation of a module (Manifest CR), the synchronization of the Kyma CR, or the installation of Watcher.
- `Warning`: Indicates that the Kyma installation is waiting for a situation to be resolved. For example, a finalizer blocks the uninstallation of a module. Typically, the user must resolve a long-running `Warning` state.
- `Error`: Indicates a technical problem that must be resolved. For example, when Lifecycle Manager is unable to connect to a Kyma runtime instance. Typically, technical support must resolve a long-running `Error` state.
- `Deleting`: Indicates that the Kyma installation is being deleted.

The **state** is always reported based on the last reconciliation loop of the [Kyma controller](https://github.com/kyma-project/lifecycle-manager/blob/main/internal/controller/kyma/controller.go). It will be set to `Ready` only if all installations were successfully executed and are consistent at the time of the reconciliation.

### **.status.conditions**

The conditions represent individual elements of the reconciliation that can either be `true` or `false`, for example, representing the readiness of all modules (Manifest CRs). For more details on how conditions are aggregated and built, take a look at [KEP-1623: Standardize Conditions](https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/1623-standardize-conditions), which is our reference documentation for conditions.

Currently, we maintain conditions for:

* All modules (Manifest CRs) that are in the `Ready` state
* Module catalog (ModuleTemplate CR and ModuleReleaseMeta CR) synchronized to the remote cluster
* Watcher installed in the remote cluster

We also calculate the **.status.state** readiness based on all the conditions available.

### **.status.modules**

This describes the tracked modules that should be installed within a Kyma cluster. Each tracked module is based on one entry in **.spec.modules** and represents the resolved Manifest CR that is based on a given ModuleTemplate CR in a release channel:

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Kyma
# ...
status:
  modules:
  - channel: regular
    fqdn: kyma.project.io/module/btp-operator
    manifest:
      apiVersion: operator.kyma-project.io/v1beta2
      kind: Manifest
      metadata:
        generation: 1
        name: 24bd3cbf-454a-4075-baa6-113a23fdfcd0-btp-operator-4128321918
        namespace: kcp-system
    name: btp-operator
    state: Processing
    template:
      apiVersion: operator.kyma-project.io/v1beta2
      kind: ModuleTemplate
      metadata:
        generation: 2
        name: moduletemplate-btp-operator
        namespace: kcp-system
    version: 1.2.10
```

The above example shows that not only is the module name resolved to a unique `fqdn`, it also represents the active `channel`, `version`, and `state`, which is a direct tracking to the **.status.state** in the Manifest CR. The Kyma CR `Ready` state can only be achieved if all tracked modules are `Ready` themselves.

The Manifest CR can be directly observed by looking at the **metadata**, **apiVersion**, and **kind**, which can be used to dynamically resolve the module.

The same is done for the ModuleTemplate CR. The actual one that is used as a template to initialize and synchronize the module, similarly, is referenced by **apiVersion**, **kind**, and **metadata**.

To observe not only how the state of the `synchronization` but the entire reconciliation is working, as well as to check on latency and the last observed change, we also introduce the **lastOperation** field. This contains not only a timestamp of the last change (which allows you to view the time since the module was last reconciled by Lifecycle Manager), but also a message that either contains a process message or an error message in case of an `Error` state. Thus, to get more details of any potential issues, it is recommended to check **lastOperation**.

In addition, we also regularly issue Events for important things happening at specific time intervals, e.g., critical errors that ease observability.

## `operator.kyma-project.io` Labels

Various overarching features can be enabled/disabled or provided as hints to the reconciler by providing a specific label key and value to the Kyma CR and its related resources. For better understanding, use the matching [API label reference](https://github.com/kyma-project/lifecycle-manager/blob/main/api/shared/operator_labels.go).

The most important labels include, but are not limited to:

* `operator.kyma-project.io/kyma-name`: The `runtime-id` of the Kyma runtime instance.
* `operator.kyma-project.io/skip-reconciliation`: A label that can be used with the value `true` to completely disable reconciliation for a Kyma CR. It can also be used in the Manifest CR to disable a specific module. This disables all reconciliations for the entire Kyma or Manifest CRs. Note that even if reconciliation is disabled for the Kyma CR, the Manifest CR in a Kyma can still be reconciled normally if not adjusted to have the label set as well.
* `operator.kyma-project.io/managed-by`: A cache limitation label that must be set to `lifecycle-manager` to have the resources picked up by the cache. Hard-coded but will be made dynamic to allow for multi-tenant deployments that have non-conflicting caches
* `operator.kyma-project.io/internal`: A boolean value. If set to `true`, the ModuleTemplate CRs labeled with the same label, so-called `internal` modules, are also synchronized with the remote cluster. The default value is `false`.
* `operator.kyma-project.io/beta`: A boolean value. If set to `true`, the ModuleTemplate CRs labeled with the same label, so-called `beta` modules, are also synchronized with the remote cluster. The default value is `false`.

## Annotations

* `skr-domain`: The domain of the Kyma runtime instance.
* `kyma-[kcp|skr]-crd-generation`: The generation of the Kyma CRD in both KCP and the Kyma runtime instance. Used to determine if the CRD must be updated in the Kyma runtime instance.
* `modulereleasemeta-[kcp|skr]-crd-generation`: The generation of the ModuleReleaseMeta CRD in both KCP and the Kyma runtime instance. Used to determine if the CRD must be updated in the Kyma runtime instance.
* `moduletemplate-[kcp|skr]-crd-generation`: The generation of the ModuleTemplate CRD in both KCP and the Kyma runtime instance. Used to determine if the CRD must be updated in the Kyma runtime instance.

## `operator.kyma-project.io` Finalizers

* `operator.kyma-project.io/Kyma`: A finalizer set by Lifecycle Manager to handle the Kyma CR cleanup.
* `operator.kyma-project.io/purge-finalizer`: A finalizer set by Lifecycle Manager to handle the purge of Kyma runtime's resources when the Kyma CR is deleted.
* `operator.kyma-project.io/runtime-monitoring-finalizer`: A finalizer set by Runtime Monitoring.
