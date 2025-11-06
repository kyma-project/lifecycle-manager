# ModuleReleaseMeta 

The `modulereleasemetas.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to configure the ModuleReleaseMeta resource.

The ModuleReleaseMeta custom resource (CR) describes the channel-version pairs for modules. Each ModuleReleaseMeta resource represents one module and defines the available channels for this module along with the version that is currently assigned to each of the channels.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd modulereleasemetas.operator.kyma-project.io -o yaml
```

> [!Note]
> The ModuleReleaseMeta CR is applied in both Kyma Control Plane (KCP) and SAP BTP, Kyma runtime clusters.
> Lifecycle Manager synchronizes the ModuleReleaseMeta from KCP to the applicable Kyma runtime instances.

## Configuration

### **.spec.moduleName**

The **moduleName** defines the name of the module for which the channel assignments are listed.

### **.spec.beta**

The **beta** flag defines if the module is a `beta` module. If marked as `beta`, it is only synced to Kyma runtimes where the Kyma CR is marked with the `"operator.kyma-project.io/beta": "true"` label. This includes the ModuleTemplates related to this module.

The default value is `false`.

### **.spec.internal**

The **internal** flag defines if the module is an `internal` module. If marked as `internal`, it is only synced to Kyma runtimes where the Kyma CR is marked with the `"operator.kyma-project.io/internal": "true"` label. This includes the ModuleTemplates related to this module.

The default value is `false`.

### **.spec.channels**

The **channels** define each module channel with its corresponding version. Each channel can only have one version assigned.
See the following example:

```yaml
spec:
  moduleName: keda
  beta: false
  internal: false
  channels:
    - channel: regular
      version: 1.0.0
    - channel: experimental
      version: 1.1.0
    - channel: fast
      version: 1.1.0
```

## `operator.kyma-project.io` Finalizer

* `operator.kyma-project.io/mandatory-module`: A finalizer set by Lifecycle Manager to handle the mandatory module's cleanup.
