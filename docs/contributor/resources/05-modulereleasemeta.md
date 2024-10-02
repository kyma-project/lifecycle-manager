# ModuleReleaseMeta 

The `modulereleasemetas.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to configure the ModuleReleaseMeta resource.

The ModuleReleaseMeta custom resource (CR) describes the channel-version pairs for modules. Each ModuleReleaseMeta represents one module and defines the available channels for this module along with the version that is currently assigned to the channel.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd modulereleasemetas.operator.kyma-project.io -o yaml
```

## Configuration

### **.spec.moduleName**

The **moduleName** defines the name of the module for which the channel assignments are listed.

### **.spec.channels**

The **channels** define each module channel with its corresponding version. Each channel can only have one version assigned.
See the following example:

```yaml
spec:
  moduleName: keda
  channels:
    - channel: regular
      version: 1.0.0
    - channel: experimental
      version: 1.1.0
```

