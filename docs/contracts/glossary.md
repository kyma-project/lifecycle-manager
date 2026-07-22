### Definitions

SKR - remote cluster (a concrete instance) managed by Lifecycle-Manager.

Module-CRD - The Custom Resource Definition (CRD) for the API group used by the `Default-Module-CR`

Default-Module-CR - The Custom Resource (CR) defined statically by the Module Team in the Module OCM artifact.
             Also called `Default CR` in the context of the Lifecycle-Manager reconciliation process.
             `Default-Module-CR` API type is definition is provided by the `Module-CRD`.
             Depending on Module's specific requirements, in the SKR there may exist multiple Custom Resources of the `Module-CRD` type,
             that are collectively called `Module-CRs`. Regardless of the number of `Module-CRs` existing in the SKR,
             only the one defined in the Kyma Module OCM artifact is considered to be the `Default-Module-CR`.

Module-CR -  Any Custom Resource of the `Module-CRD` type existing in the SKR. Also includes the `Default-Module-CR`
