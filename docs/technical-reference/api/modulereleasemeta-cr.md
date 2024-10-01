# ModuleReleaseMeta Custom Resource

The [ModuleReleaseMeta custom resource (CR)](../../../api/v1beta2/modulereleasemeta_types.go) is used to represent the version-channel pairs for modules. Each item represents 
a module version along with its assigned channel.

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

