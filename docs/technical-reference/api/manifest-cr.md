# Manifest Custom Resource

The [Manifest custom resource (CR)](../../../api/v1beta2/manifest_types.go) is our internal representation of what results from the resolution of a ModuleTemplate CR in the context of a single cluster represented by a Kyma CR. Thus, a lot of configuration elements are similar or entirely equivalent to the data layer found in a ModuleTemplate CR.

## Configuration

### **.spec.remote**

This parameter determines whether the given module should be installed in a remote cluster or not. If it should, then in the cluster it will attempt to search for a Secret with the `operator.kyma-project.io/kyma-name` label having the same value as the `operator.kyma-project.io/kyma-name` label in the Manifest CR.

Thus a Manifest CR like

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Manifest
metadata:
  labels:
    operator.kyma-project.io/kyma-name: kyma-sample
  name: manifest-sample
  namespace: default
spec:
  remote: true
```

will use the `kyma-sample` value to look for a Secret with the same `kyma-sample` value.

## **.spec.config**

The config reference uses an image layer reference that contains configuration data that can be used to further influence any potential rendering process while the resources are processed by the [declarative library](../../../internal/declarative/README.md#resource-rendering). It is resolved through a translation of the ModuleTemplate CR to the Manifest CR during the [resolution of the modules](../../../pkg/module/parse/template_to_module.go) in the Kyma CR control loop.

There can be at most one config layer, and it is referenced by the **name** `config` with **type** `yaml` as `localOciBlob` or `OCIBlob`:

```yaml
spec:
  descriptor:
    component:
      componentReferences: []
      name: kyma-project.io/module/keda
      provider: internal
      repositoryContexts:
      - baseUrl: europe-docker.pkg.dev/kyma-project/prod/unsigned
        componentNameMapping: urlPath
        type: ociRegistry
      resources:
      - access:
          digest: sha256:f4a599c4310b0fe9133b67b72d9b15ee96b52a1872132528c83978239b5effef
          type: localOciBlob
        name: config
        relation: local
        type: yaml
        version: 0.0.1-6cd5086
```

### **.spec.install**

The installation layer contains the relevant data required to determine the resources for the [renderer during the manifest reconciliation](../../../internal/declarative/README.md#resource-rendering).

It is mapped from an access type layer in the descriptor:

```yaml
- access:
    digest: sha256:8f926a08ca246707beb9c902e6df7e8c3e89d2e75ff4732f8f00c424ba8456bf
    type: localOciBlob
  name: keda
  relation: local
  type: helm-chart
  version: 0.0.1-6cd5086
```

will be translated into a Manifest Layer:

```yaml
install:
   name: keda
   source:
      name: kyma-project.io/module/keda
      ref: sha256:8f926a08ca246707beb9c902e6df7e8c3e89d2e75ff4732f8f00c424ba8456bf
      repo: europe-docker.pkg.dev/kyma-project/prod/unsigned/component-descriptors
      type: oci-ref
```

The [internal spec resolver](../../../internal/manifest/spec_resolver.go) uses this layer to resolve the correct specification style and renderer type from the data layer.

### **.spec.resource**

The resource is the default data that should be initialized for the module and is directly copied from **.spec.data** of the ModuleTemplate CR after normalizing it with the **namespace** for the synchronized module.

### **.status**

The Manifest CR status is an unmodified version of the [declarative status](../../../internal/declarative/README.md#resource-tracking), so the tracking process of the library applies. There is no custom API for this.

### **.metadata.labels**

* `operator.kyma-project.io/skip-reconciliation`: A label that can be used with the value `true` to disable reconciliation for a module. This will avoid all reconciliations for the Manifest CR.
