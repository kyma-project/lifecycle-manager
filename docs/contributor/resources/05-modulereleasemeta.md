# ModuleReleaseMeta Custom Resource

`modulereleasemetas.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to configure the ModuleReleaseMeta resource.

The ModuleReleaseMeta custom resource is used to represent the version-channel pairs for modules. Each item represents
a module version along with its assigned channel.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd modulereleasemetas.operator.kyma-project.io -o yaml
```

## Configuration

### **.spec.moduleName**

This parameter defines the name of the module for which the channel assignments are.

### **.spec.channels**

This parameter defines each channel with its corresponding version for the module. Each channel can only have one version assigned.
An example could be:

```yaml
spec:
  moduleName: keda
  channels:
    - channel: regular
      version: 1.0.0
    - channel: experimental
      version: 1.1.0
```

