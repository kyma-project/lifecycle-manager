# Maintenance Windows

Lifecycle Manager allows a module team to specify if a version of their module requires downtime during the version upgrade. This is specified using the **spec.requiresDowntime** field in the ModuleTemplate custom resource (CR).
Additionally, an SAP BTP, Kyma runtime user can opt in to using maintenance windows by setting the **spec.skipMaintenanceWindow** field to `false` in the Kyma CR.

## Scenarios

| **requiresDowntime** | Description |
|----------------------|-------------|
| `true` | - If the module version is available for upgrade and the maintenance window is active, then the Kyma module is upgraded to the new version.<br>- If the module version is available for upgrade and the maintenance window is not yet active, then the Kyma module is not upgraded and remains reconciled with the current version till the next maintenance window. |
|`false` | Whenever the module version is available for upgrade, the Kyma module is upgraded to the new version immediately. |