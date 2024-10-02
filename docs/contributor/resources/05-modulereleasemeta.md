# ModuleReleaseMeta 

The `modulereleasemetas.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to configure the ModuleReleaseMeta resource.

The ModuleReleaseMeta custom resource (CR) describes the channel-version pairs for modules. Each entry represents a module channel along with its assigned version. Each module requires a separate dedicated ModuleReleaseMeta CR.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd modulereleasemetas.operator.kyma-project.io -o yaml
```

## Configuration

### **.spec.moduleName**

This parameter defines the name of the module for which the channel assignments are listed.

### **.spec.channels**

This parameter defines each channel with its corresponding module version. Each channel can only have one version assigned.
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

