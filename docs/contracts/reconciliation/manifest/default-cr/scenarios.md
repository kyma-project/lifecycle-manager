# Scenarios

## S01: Non-Manadatory module installation

### Initial conditions
There is a non-mandatory module `foo`in the `regular` channel available in the SKR.
The module `foo` is not yet installed in the cluster.
The user adds the module `foo` in the `regular` channel to the kyma module list.

### Lifecycle-Manager actions
- LM fetches the `Default-Module-CR` resource for the module from a dedicated OCI layer \[R01\].
- LM tries to locate the CRD for the `Default-Module-CR` (`Module-CRD`) in the Module's resources.
- LM installs the `Module-CRD` in the SKR, if found.
- LM installs the `Default-CR` in the SKR.

### Expected results
- The `Module-CRD` is installed in the SKR.
- The `Default-Module-CR` resource is installed in the SKR.

### Alternative flows/Errors
- If the LM fails to fetch the `Default-Module-CR` resource for the module from OCI layer, it should put the manifest in the `Error` state.

### Additional notes:
A corner case of not finding the `Module-CRD` in the module's resources is not considered to be an error. Perhaps the Module Team ensures existence of the CRD in the SKR by other means.
If the `Module-CRD` is not defined in the SKR, the `Default-Module-CR` resource installation will fail anyway.

## S02: Module Team changes API version of the Default-Module-CR in a new module version

### Initial conditions
There is a non-mandatory module `foo` installed in the SKR in the `0.0.1` version.
The `Default-Module-CR` for the module installed in the SKR declares `v1alpha1` API version.
The `Module-CRD` for the module `foo` version `0.0.1` defines `v1alpha1` API version.
The Module Team releases version `0.0.2` of the module `foo`. This version adds `v1beta1` API version to the `Module-CRD`.
In addition, the new module version changes the `Default-Module-CR` resource definition to the new `v1beta1` API version.

### Lifecycle-Manager actions
- LM upgrades the `Module-CRD` in the SKR.
- LM does NOT rewrite or re-apply the `Default-CR` resource in the SKR, since it is not actively reconciled \[R03\].

### Expected results
- The `Module-CRD` is updated in the SKR, both API versions: `v1alpha1` and `v1beta1` are present.
- The `Default-CR` for the module in the SKR remains in the `v1alpha1` API version.

### Additional notes:
The LM is not responsible for migration of the `Default-Module-CR` resource to the new API version.
The Module Team is responsible for providing a migration path for the `Default-Module-CR` resource in the SKR \[R03\].

