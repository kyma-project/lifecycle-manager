# ModuleConfig Custom Resource

The [ModuleConfig custom resource (CR)](../../../api/v1alpha1/moduleconfig_types.go) facilitates the synchronization of the Module CR configuration between Kyma runtime and Kyma Control Plane clusters, reflecting dynamic changes made by the user.

## Generation

[Runtime Watcher](https://github.com/kyma-project/runtime-watcher) notifies Lifecycle Manager (LM) of changes detected in a Module CR. Lifecycle Manager generates or updates a ModuleConfig CR in Kyma runtime, based on the detected changes.

## Prerequisites

For Lifecycle Manager to generate a ModuleConfig CR, it requires the following prerequisites:
- [Runtime Watcher](https://github.com/kyma-project/runtime-watcher) enabled
- The **.spec.enableModuleConfig** parameter in the ModuleTemplate CR set to `true`. To set this field, use the `--enableModuleConfig` CLI flag when generating `ModuleTemplate`.

## Workflow

The workflow for a ModuleConfig CR involves several steps, orchestrated by Lifecycle Manager and Runtime Watcher:

1. The end-user updates an existing Module CR in Kyma runtime. 


2. Runtime Watcher notifies Lifecycle Manager about the changes detected in the Module CR.

3. Lifecycle Manager fetches the latest version of the Module CR from the Kyma runtime.

4. Lifecycle Manager generates or updates a ModuleConfig CR from the Module CR in the Kyma runtime.

5. Module Manager watches the ModuleConfig CR for changes.

6. Module Manager generates a SyncResource CR from the ModuleConfig CR enabling  Lifecycle Manager to reconcile the end-user's changes.


![Sync Resource Sequence Diagram](../../assets/sync-resource-sequence.svg)

## Configuration
### **.spec.kyma** and **.spec.module**
These are mandatory fields specifying the names of the target `Kyma` and `Module` on the SKR from which this `ModuleConfig` originates.

The following `ModuleConfig` belongs to the `sample-yaml` module related to the `kyma-sample-dzeas` kyma:
```yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: ModuleConfig
metadata:
  labels:
    "operator.kyma-project.io/kyma-name": kyma-sample-dzeas
    "operator.kyma-project.io/module-name": sample-yaml
  name: moduleconfig-sample
  namespace: kcp-system
spec:
  kyma: kyma-sample-dzeas
  module: sample-yaml
```

### .spec.resource

This field contains the `Module` CR YAML content as plain text, including the subresource (status) field.
It reflects the configuration done by the user and used by the MM to generate the corresponding `SyncResource` CR.

E.g., The following `ModuleConfig` syncs the sizes of NFS volumes configured by the user is the SKR:
```yaml
spec:
  resource: |
      apiVersion: operator.kyma-project.io/v1alpha1
      kind: Infrastructure
      metadata:
        name: default
        namespace: kyma-system
      spec:
        vpcPeerings:
          - name: peering-1
            description: peering-1
            remoteVpcId: vpc-1
            remoteRegion: eu-central-1
            remoteCidrRange:
          - name: peering-2
            description: peering-2
            remoteVpcId: vpc-2
            remoteRegion: eu-central-1
            remoteCidrRange:
        nfsVolumes:
          - name: nfs-1
            description: nfs-1
            size: 100Gi
          - name: nfs-2
            description: nfs-2
            size: 100Gi
       status:
          ...
          state: Ready
```

### `operator.kyma-project.io` labels:

These are the labels available on the `ModuleConfig` CR:
- `operator.kyma-project.io/kyma-name`: string. It enables the Module manager to identify the target Kyma runtime where this config coming from.
- `operator.kyma-project.io/module-name`: string. It provides the individal Module manager to filter the ModuleConfig which belongs to it.
