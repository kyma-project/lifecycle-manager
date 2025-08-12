# ADR 006 - Upgrade Strategy for Custom Resource Definition (CRD) Version in Managed Mode

## Status

Accepted

## Context

We currently have several issues that require introducing new spec fields to existing CRDs in our system. Typically, if these spec fields do not introduce breaking changes, such as being optional fields, we can add them using the same CRD version. 

However, in managed mode, we need to sync Kyma to remote, and the remote CRD does not get updated if they are in the same version, which means that remote Kyma instances will not contain changes introduced by new spec fields if we are not updating the CRD version in KCP.

This has raised a concern about whether we should keep the current logic of requiring a version upgrade for any change, or introduce a process to detect differences even if CRDs are in the same version, to avoid unnecessary version upgrades.

Two options for non-breaking changes:
1. Keep current logic, when new change comes, create new version, set storage: true, served: false, which means content will be saved as new version, but the resources will be presented still using old version. When all changes are implemented, change served:true for new version.
2. Introduce a process to detect differences between KCP and SKR even if CRDs are in the same version, and update SKR CRD if necessary. 

## Decision

The decision is to implement option 2, which is to introduce a process to detect differences between KCP and SKR even if CRDs are in the same version, and update SKR CRDs if necessary.

Record existing CRDs generation numbers, both SKR and KCR CRDs, check for differences in CRD generation numbers, and if there are any, apply CRD updates to SKR. We use Kyma CR annotations to save CRD generation numbers.

We implemented the decision [here](https://github.com/kyma-project/lifecycle-manager/issues/534).

## Consequences

The implementation of the process to detect differences in CRDs and update SKR CRDs will ensure that changes introduced in KCP CRDs are properly reflected in remote SKR instances, even if the CRDs are in the same version. This will prevent unnecessary version upgrades and ensure consistency between KCP and SKR CRDs. However, it may introduce additional overhead in terms of performance due to the comparison and update process, which will need to be carefully evaluated during performance testing.
