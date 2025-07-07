# Setting Your Module to the Unmanaged and Managed State

In some cases, for example, for testing, you may need to modify your module beyond what is supported by its configuration. By default, when a module is in the managed state, Kyma Control Plane governs its Kubernetes resources, reverting any manual changes during the next reconciliation loop. To modify Kubernetes objects directly without them being reverted, you must set the module to the unmanaged state. In this state, reconciliation is disabled, ensuring your manual changes are preserved.

To unmanage a module, set the **.spec.modules[].managed** field to `false` in the Kyma CR. The following changes are then triggered:

* The module and all its related resources remain in the SKR cluster in the same state they were in when the module became unmanaged.
* The module and its resources stop being reconciled in the KCP cluster.
* The `operator.kyma-project.io/managed-by=kyma` and `operator.kyma-project.io/watched-by=kyma` labels are removed from the module's resources. For example, this may be relevant if you use those labels as exclusion filters for custom monitoring using the Kyma Telemetry module.

To verify that a module was successfully unmanaged, check that the field **.status.modules[].state** has the status `Unmanaged`. Once the state is `Unmanaged`, you can delete the module's entry from **.spec.modules[]** in the Kyma CR.  Nevertheless, the module and its related resources remain in the remote cluster.

> [!Warning]
> When you switch values of **.spec.modules[].managed**, you MUST wait for the new state to be reflected in **.status.modules[].state** before you remove the module's entry from **.spec.modules[]**. If the entry is removed before the current state is reflected properly in **.status.modules[].state**, it may lead to unpredictable behavior that is hard to recover from.

When the **.spec.modules[].managed** field is set back to `true`, Lifecycle Manager starts the module management again. The existing module resources in the remote cluster may be overwritten if the desired state has changed in the meantime, for example, if the module's version within the used channel was updated.

> [!Warning]
> Setting a module back to the managed state does not guarantee its version is correctly updated.

For more information, see [Setting Your Module to the Unmanaged and Managed State](https://help.sap.com/docs/btp/sap-business-technology-platform/setting-your-module-to-unmanaged-state?version=Cloud).