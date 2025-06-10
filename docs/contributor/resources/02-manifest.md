# Manifest

The `manifests.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to configure the Manifest resource.

The Manifest custom resource (CR) represent resources that make up a module and are to be installed by Lifecycle Manager. The Manifest CR is a rendered module added to a particular cluster.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd manifests.operator.kyma-project.io -o yaml
```

## Patching

The [Runner](../../../pkg/module/sync/runner.go) is responsible for creating and updating Manifest CRs using Server Side Apply (SSA). An update is only performed when one of the following conditions is met:

1. The Manifest CR version differs from the Kyma CR's module status version.
2. The Manifest CR channel differs from the Kyma CR's module status channel.
3. The Manifest CR state differs from the Kyma CR's module status state.

## Configuration

### **.spec.install**

This contains the OCI resource specification for the module resources that will be deployed on the target cluster.

The following example shows how the `raw-manifest` is defined in the Manifest CR:

```yaml
spec:
  install:
    name: raw-manifest
    source:
      name: kyma-project.io/module/btp-operator
      ref: sha256:5e9436f2b6b90667415aec3809af73d4c884d8f275e21958433103188f661d4c
      repo: europe-docker.pkg.dev/kyma-project/modules-internal/component-descriptors
      type: oci-ref
```

This specification is mapped from the corresponding access layer in the ModuleTemplate CR descriptor:

```yaml
- access:
    localReference: sha256:5e9436f2b6b90667415aec3809af73d4c884d8f275e21958433103188f661d4c
    mediaType: application/octet-stream
    referenceName: raw-manifest
    type: localBlob
  digest:
    hashAlgorithm: SHA-256
    normalisationAlgorithm: genericBlobDigest/v1
    value: 5e9436f2b6b90667415aec3809af73d4c884d8f275e21958433103188f661d4c
  name: raw-manifest
  relation: local
  type: yaml
  version: 1.2.10
```

The `.spec.install` spec is resolved by the [SpecResolver](../../../internal/manifest/spec_resolver.go) which fetches the raw manifest from the OCI layer and resolves it to a path that can be used by the [declarative library](../../../internal/declarative/) and deployed to the target cluster.


### **.spec.resource**

The resource is the default data that should be initialized for the module and is directly copied from **.spec.data** of the ModuleTemplate CR after normalizing it with the **namespace** for the synchronized module.

### **.status**

The Manifest CR status is set based on the following logic, managed by the manifest reconciler:

* If the module defined in the Manifest CR is successfully applied and the deployed module is up and running, the status of the Manifest CR is set to `Ready`.
* While the manifest is being applied and the Deployment is still starting, the status of the Manifest CR is set to `Processing`.
* If the Deployment cannot start (for example, due to an `ImagePullBackOff` error) or if the application of the manifest fails, the status of the Manifest CR is set to `Error`.
* If the Manifest CR is marked for deletion, the status of the Manifest CR is set to `Deleting`.

This status provides a reliable way to track the state of the Manifest CR and the associated module. It offers insights into the deployment process and any potential issues while being decoupled from the module's business logic.

### **.metadata.labels**

* `operator.kyma-project.io/skip-reconciliation`: A label that can be used with the value `true` to disable reconciliation for a module. This will avoid all reconciliations for the Manifest CR. Note that this label is independent of the Kyma CR's skip reconciliation label. 

### **Unused Fields**

#### **.spec.remote**

This parameter was used to determine whether the given module should be installed in a remote cluster. If it should, then in the KCP cluster, it attempts to search for a Secret having the same `operator.kyma-project.io/kyma-name` label and value as in the Manifest CR. This is the default and only behaviour now.

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

#### **.spec.config**

The config reference used an image layer reference that contains configuration data that could be used to further
influence any potential rendering process while the resources are processed by
the [declarative library](../../../internal/declarative/). It was resolved through a
translation of the ModuleTemplate CR to the Manifest CR during
the [resolution of the modules](../../../internal/manifest/parser/template_to_module.go) in the Kyma CR control loop.

Now, only raw manifests are supported, and the config layer is no longer used.

There could have been at most one config layer, and it was referenced by the **name** `config` with **type** `yaml` as `localOciBlob` or `OCIBlob`:

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
