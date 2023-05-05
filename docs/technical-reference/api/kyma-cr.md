# Kyma Custom Resource

The [Kyma](../../../api/v1beta1/kyma_types.go) custom resource (CR) contains 3 fields that are together used to declare the desired state of a cluster:

1. `.spec.channel` and `.spec.modules[].channel`: The Release Channel that should be used by default for all modules that are to be installed in the cluster.
2. `.spec.modules`: The Modules that should be installed into the cluster. Each Module contains a name (which we will get to later) serving as a link to a `ModuleTemplate`. 
Additionally one can add a specific channel (if `.spec.channel`) should not be used. 
On Top of that one can specify a `controller`, which serves as a Multi-Tenant Enabler. 
It can be used to only listen to ModuleTemplates provided under the same controller-name. Last but not least, it includes a `customResourcePolicy` which can be used for specifying defaulting behavior when initialising modules in a cluster.

### `.spec.channel` and `.spec.modules[].channel`

The `.spec.channel` attribute is used in conjunction with the [release channels](https://github.com/kyma-project/community/tree/main/concepts/modularization#release-channels). The channel that is used for a Kyma resource will always be used as the default in case no other specific channel is used.

This means that a Kyma that has the spec

```yaml
spec:
  channel: alpha
  modules:
  - name: keda
  - name: serverless
```

will attempt to look up the modules `keda` and `serverless` in the `alpha` release channel.

However, when specifying channels specifically through `.spec.modules[].channel`, the latter one is used instead.

```yaml
spec:
  channel: alpha
  modules:
  - name: keda
    channel: regular
  - name: serverless
```

In this case the relevant channel will be `regular` for `keda`, but not for `serverless` 

### `.spec.modules`

The module list is used to define the desired set of all modules. This is mainly derived from the attribute `.spec.modules[].name` which is resolved in one of 3 ways.

Let's take a look at this simplified ModuleTemplate:
```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: ModuleTemplate
metadata:
  name: moduletemplate-sample
  namespace: default
  labels:
    "operator.kyma-project.io/module-name": "module-name-from-label"
spec:
  channel: regular
  data: {}
  descriptor:
    component:
      name: kyma-project.io/module/sample
```

The module mentioned above can be referenced in one of 3 ways
1. The label value of `operator.kyma-project.io/module-name`
    ```yaml
    spec:
      channel: regular
      modules:
      - name: module-name-from-label
    ```
2. The Name or Namespace/Name of a ModuleTemplate
    ```yaml
    spec:
      channel: regular
      modules:
      - name: moduletemplate-sample
    ```
   or
    ```yaml
    spec:
      channel: regular
      modules:
      - name: default/moduletemplate-sample
    ```
3. The Fully Qualified Name of the descriptor as located in `.spec.descriptor.component.name`
    ```yaml
    spec:
      channel: regular
      modules:
      - name: kyma-project.io/module/sample
    ```

### `.spec.modules[].customResourcePolicy`

In addition to this very flexible way of referencing modules, there is also another flag that can be important for users requiring more flexibility during module initialization: `customResourcePolicy` is used to define one of `CreateAndDelete` and `Ignore`. 
While `CreateAndDelete` will cause the ModuleTemplate's `.spec.data` to be created and deleted to initialize a module with preconfigured defaults, `Ignore` can be used to only initialize the operator without initializing any default data. 
This allows users to be fully flexible when and how to initialize their Module.

### `.status.state`

The `State` is a simple representation of the state of the entire `Kyma` installation. It is defined as an aggregated status that is one of `Ready`, `Processing`, `Error` and `Deleting`, based on the status of _all_ `Manifest` resources on top of the validity/integrity of the synchronization to a remote cluster if enabled.

The `State` will always be reported based on the last reconciliation loop of the [Kyma controller](../controllers/kyma_controller.go). It will be set to `Ready` only if all installations were succesfully executed and are consistent at the time of the reconciliation.

### `.status.conditions`

The conditions represent individual elements of the reconciliation that can either be `true` or `false`, for example representing the readiness of all Modules (`Manifest`). For more details on how conditions are aggregated and built, take a look at [KEP-1623: Standardize Conditions](https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/1623-standardize-conditions), which is our reference documentation for conditions.

Currently, we maintain conditions for
- Module (`Manifest`) synchronization
- Module Catalog (`ModuleTemplate`) synchronization
- Watcher Installation Consistency

We also calculate `.status.state` readiness based on all the conditions available.

### `.status.modules`

This describes the tracked modules that should be installed within a `Kyma` cluster. Each tracked module is based on one entry in `.spec.modules` and represents the resolved `Manifest` that is based on a given `ModuleTemplate` in a release `channel`:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Kyma
# ...
status:
  modules:
  - channel: alpha
    fqdn: kyma.project.io/module/btp-operator
    manifest:
      apiVersion: operator.kyma-project.io/v1beta1
      kind: Manifest
      metadata:
        generation: 1
        name: 24bd3cbf-454a-4075-baa6-113a23fdfcd0-btp-operator-4128321918
        namespace: kcp-system
    name: btp-operator
    state: Processing
    template:
      apiVersion: operator.kyma-project.io/v1beta1
      kind: ModuleTemplate
      metadata:
        generation: 2
        name: moduletemplate-btp-operator
        namespace: kcp-system
    version: v0.2.3
```

As can be seen above, not only is the module name resolved to a unique `fqdn`, it also represents the active `channel`, `version` and `state` which is a direct tracking to the `Manifest` `.status.state`. The Kyma `Ready` state can only be achieved if all tracked modules are in `Ready` themselves.

The `Manifest` can be directly observed by looking at the `metadata` and `apiVersion` and `kind` which can be used to dynamically resolve the module.

The same is done for the `ModuleTemplate`. The actual one that is used as template to initialize and synchronize the module similarly is referenced by `apiVersion`, `kind` and `metadata`.

To observe not only how the state of the `synchronization` but the entire reconciliation is working, as well as to check on latency and the last observed change, we also introduce a new field: `lastOperation`. This contains not only a timestamp of the last change (which allows to view the time since the module was last reconciled by Lifecycle Manager), but also a message that either contains a process message, or an error message in case of an `Error` state. Thus, to get more details of any potential issues, it is recommended to check `lastOperation`.

In addition to this we also regularly issue `Events` for important things happening at specific time intervals, e.g. critical errors that ease observability.

### Label-based side-effects from `operator.kyma-project.io`

There are various overarching features that can be enabled/disabled or provided as hints to the reconciler by providing a specific label key and value to the `Kyma` and it's related resources. To take a look at labels, use the matching [API label reference](v1beta1/operator_labels.go).

The most important ones include, but are not limited to:

- `operator.kyma-project.io/Kyma`: the [finalizer](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) set by lifecycle manager to deal with `Kyma` cleanup
- `operator.kyma-project.io/kyma-name`: An identifier that can be set on a secret to identify correct cluster access kubeconfigs to be used during reconciliation.
- `operator.kyma-project.io/signature`: An identifier that can be set on a secret to identify correct signature X.509 Secrets that contain a key called `key` which contains a X.509 PKIX PublicKey or an PKCS1 Public Key. Used in conjunction with the label-value for templates signed with a signature in the descriptor.
- `operator.kyma-project.io/skip-reconciliation`: A label that can be used with value `true` to completely disable reconciliation for a `Kyma` resource. Can also be used on the `Manifest` to disable a specific module. This will avoid all reconciliations for the entire Kyma `Resource` or `Manifest` resource. Note that even though reconciliation for `Kyma` might be disabled, the `Manifest` in a Kyma can still get reconciled normally if not adjusted to have the label set as well.
- `operator.kyma-project.io/managed-by`: A cache limitation label that must be set to `lifecycle-manager` to have the resources be picked up by the cache. Hard-coded but will be made dynamic to allow for multi-tenant deployments that have non-conflicting caches
- `operator.kyma-project.io/purpose`: Can be used to identify resources by their intended purpose inside lifecycle manager. Useful meta information for cluster managers.