# Scenarios

## S01: Non-Manadatory module installation

### Initial conditions
There is a non-mandatory module `foo`in the `regular` channel available in the SKR.
The module `foo` is not yet installed in the cluster.
The user adds the module `foo` in the `regular` channel to the kyma module list.

### Lifecycle-Manager actions
- LM fetches the `Default-CR` resource for the module from a dedicated OCI layer.
- LM locates the CRD for the `Default-CR` API group (`Module-CRD`) in the Module's resources.
- LM installs the `Module-CRD` in the SKR
- LM installs the `Default-CR` in the SKR.

### Expected results
- The `Module-CRD` is installed in the SKR.
- The `Default-CR` resource is installed in the SKR.

### Alternative flows/Errors
- If the LM fails to fetch the `Default-CR` resource for the module from OCI layer, it should put the manifest in the `Error` state.
- If the LM fails to locate the `Module-CRD` in the Module's resources, it should put the manifest in the `Error` state.


## S02: Module Team changes API version of the Default-CR in a new module version

### Initial conditions
There is a non-mandatory module `foo` installed in the SKR in the `0.0.1` version.
The `Default-CR` for the module installed in the SKR declares `v1alpha1` API version.
The `Module-CRD` for the module `foo` version `0.0.1` also defines `v1alpha1` API version.
The Module Team releases version `0.0.2` of the module `foo`. This version adds `v1beta1` API version to the `Module-CRD`.
In addition, the new module version changes the `Default-CR` resource definition to use the new `v1beta1` API version.

### Lifecycle-Manager actions
- LM upgrades the `Module-CRD` in the SKR.
- LM does NOT rewrite or re-apply the `Default-CR` resource in the SKR, since it is not actively reconciled {R03}.

### Expected results
- The `Module-CRD` is updated in the SKR, both API versions: `v1alpha1` and `v1beta1` are present.
- The `Default-CR` for the module in the SKR is, at some point in time **migrated** to the new `v1beta1` API version by the Module's operator. LM is not responsible for this migration.

