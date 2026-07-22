### Definitions

SKR - remote cluster (a concrete instance) managed by Lifecycle-Manager

Default-CR - The default Custom Resource (CR) that is used to define the configuration for the running Kyma Module and provides feedback on Module's state in the SKR.
             Depending on Module's specific requirements, in the SKR there may exist multiple Custom Resources of the same API Group (as the `Default-CR`),
             but only the one defined statically in the Kyma Module OCM artifact is considered to be a `Default-CR`.

Module-CRD - The Custom Resource Definition (CRD) for the API group used by the `Default-CR`
