# Manifest Custom Resource

The [Manifest custom resource (CR)](../../../api/v1beta2/manifest_types.go) is our internal representation of what results from the resolution of a ModuleTemplate CR in the context of a single cluster represented by a Kyma CR. Thus, a lot of configuration elements are similar or entirely equivalent to the data layer found in a ModuleTemplate CR.

## Configuration

### **.spec.remote**

This parameter determines whether or not the given module should be installed in a remote cluster. If it should, then in the KCP cluster, it attempts to search for a Secret having the same `operator.kyma-project.io/kyma-name` label and value as in the Manifest CR.

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

looks for a Secret with the same `operator.kyma-project.io/kyma-name` label and value `kyma-sample`.

### **.spec.config**

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

### **.spec.managedResources**

The managedResources are the resources which are managed manually by the user. They are represented by the `group/version/kind` format. 
If the module is managed and gets deleted from the Kyma CR, module deletion is suspended until all custom resources with GVK listed in the spec.managedResources are removed manually by the user.   
```yaml
spec:
  managedResources: 
  - serverless.kyma-project.io/v1alpha2/functions
  - operator.kyma-project.io/v1alpha1/serverlesses
```

### **.status**

The Manifest CR status is set based on the following logic, managed by the manifest reconciler: 

- If the module defined in the Manifest CR is successfully applied and the deployed module is up and running, the status of the Manifest CR is set to `Ready`.
- While the manifest is being applied and the Deployment is still starting, the status of the Manifest CR is set to `Processing`.
- If the Deployment cannot start (for example, due to an `ImagePullBackOff` error) or if the application of the manifest fails, the status of the Manifest CR is set to `Error`.
- If the Manifest CR is marked for deletion, the status of the Manifest CR is set to `Deleting`.

This status provides a reliable way to track the state of the Manifest CR and the associated module. It offers insights into the deployment process and any potential issues while being decoupled from the module's business logic.

### **.metadata.labels**

* `operator.kyma-project.io/skip-reconciliation`: A label that can be used with the value `true` to disable reconciliation for a module. This will avoid all reconciliations for the Manifest CR.
