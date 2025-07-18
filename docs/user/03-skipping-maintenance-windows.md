# Skipping Maintenance Windows

## Context

In SAP BTP, Kyma runtime, modules' major upgrades happen rarely, in a bigger maintenance window. In some cases, these upgrades require downtime. You can decide not to wait for a maintenance window and upgrade a module version requiering downtime as soon as it is available.

## Procedure

Set the **spec.skipMaintenanceWindow** field to `true` in the Kyma CR.
