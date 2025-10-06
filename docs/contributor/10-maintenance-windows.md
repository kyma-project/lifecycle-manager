# Maintenance Windows

Lifecycle Manager enables a module team to specify whether a version of their module requires downtime during the upgrade process. To configure a module version as requiring downtime, set the **spec.requiresDowntime** field in the ModuleTemplate custom resource (CR) to `true`.

Additionally, an SAP BTP, Kyma runtime user can decide not to wait for a maintenance window and upgrade a module version as soon as it is available by setting the **spec.skipMaintenanceWindow** field to `true` in the Kyma CR. For more information, see [Skipping Maintenance Windows](../user/03-skipping-maintenance-windows.md).

## Scenarios

Depending on the configuration, the following scenarios are possible:

1. The **requiresDowntime** field in the ModuleTemplate is set to `true` AND the user decides to use maintenance windows by setting the **spec.skipMaintenanceWindows** field to `false`:

   - If the module version is available for upgrade and the maintenance window is active, then the Kyma module is upgraded to the new version.
   - If the module version is available for upgrade and the maintenance window is not yet active, then the Kyma module is not upgraded and remains reconciled with the current version till the next maintenance window.

2. The **requiresDowntime** field in the ModuleTemplate is set to `false` OR the user decides not to wait for a maintenance window to upgrade a module by setting the **spec.skipMaintenanceWindows** field to `false`.

   Then, whenever the module version is available for upgrade, the Kyma module is immediately upgraded to the new version.

> [!Note]
> The **requiresDowntime**  parameter does not apply if the module was not installed before as there is no existing installation that breaks.

