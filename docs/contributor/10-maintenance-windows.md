# Maintenance Windows

Lifecycle Manager allows a module team to specify if a version of their module requires downtime during the version upgrade. This is specified using the **spec.requiresDowntime** field in the ModuleTemplate custom resource (CR).
Additionally, an SAP BTP, Kyma runtime user can opt in to using maintenance windows by setting the **spec.skipMaintenanceWindow** field to `false` in the Kyma CR.

If the `requiresDowntime` is set to `true` and the user opts-in to the usage of the maintenance window:
- If the module version is available for upgrade and the maintenance window is active, then the Kyma module is upgraded to the new version.
- If the module version is available for upgrade and the maintenance window is not yet active, then the Kyma module is not upgraded and remains reconciled with the current version till the next maintenance window.

If the `requiresDowntime` is set to `false` or the user does not opt-in to the usage of the maintenance window, then whenever the module version is available for upgrade, the Kyma module is upgraded to the new version immediately.