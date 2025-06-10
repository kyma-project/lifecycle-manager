# Maintenance Windows

Lifecycle Manager allows a module team to specify if a version of their module requires downtime during the version upgrade. This is specified using the **spec.requiresDowntime** field in the ModuleTemplate custom resource (CR).
Additionally, an SAP BTP, Kyma runtime user can opt in to using maintenance windows by setting the **spec.skipMaintenanceWindow** field to `false` in the Kyma CR.

## Scenarios

Depending on the configuration, the following scenarios are possible:

1. The **requiresDowntime** field in the ModuleTemplate is set to `true` AND the user opts in to using maintenance windows by setting the **spec.skipMaintenanceWindows** field to `false`:
   - If the module version is available for upgrade and the maintenance window is active, then the Kyma module is upgraded to the new version.
   - If the module version is available for upgrade and the maintenance window is not yet active, then the Kyma module is not upgraded and remains reconciled with the current version till the next maintenance window.

2. The **requiresDowntime** field in the ModuleTemplate is set to `false` OR the user does not opt in to using maintenance windows, by setting the **spec.skipMaintenanceWindows** field to `false`. 

   Then, whenever the module version is available for upgrade, the Kyma module is immediately upgraded to the new version.