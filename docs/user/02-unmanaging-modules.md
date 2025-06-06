# Unmanaging Modules

Lifecycle Manager allows you to unmanage modules, which means that the module and its related resources remain deployed in the SAP BTP, Kyma runtime (SKR) cluster but are no longer managed by Lifecycle Manager.

To unmanage a module, set the **.spec.modules[].managed** field to `false` in the Kyma CR. The following changes are then triggered:

* The module and all its related resources remain in the remote cluster in the same state they were in when the module became unmanaged.
* Lifecycle Manager stops reconciling the module and its resources.
* The `operator.kyma-project.io/managed-by=kyma` and `operator.kyma-project.io/watched-by=kyma` labels are removed from the module's resources. For example, this may be relevant if you use those labels as exclusion filters for custom monitoring using the Kyma Telemetry module.

To verify that a module was successfully unmanaged, check that the field **.status.modules[].state** has the status `Unmanaged`. Once the state is `Unmanaged`, you can delete the module's entry from **.spec.modules[]** in the Kyma CR.  Nevertheless, the module and its related resources remain in the remote cluster.

> **CAUTION:**
> When you switch values of **.spec.modules[].managed**, you MUST wait for the new state to be reflected in **.status.modules[].state** before you remove the module's entry from **.spec.modules[]**. If the entry is removed before the current state is reflected properly in **.status.modules[].state**, it may lead to unpredictable behavior that is hard to recover from.

When the **.spec.modules[].managed** field is set back to `true`, Lifecycle Manager starts the module management again. The existing module resources in the remote cluster may be overwritten if the desired state has changed in the meantime, for example, if the module's version within the used channel was updated.
