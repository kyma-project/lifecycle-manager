# Synchronization between KCP and SKR

This document explains how the Kyma Control Plane (KCP) cluster, where KLM operates, synchronizes with the user's runtime cluster, the SAP Kyma Runtime (SKR) cluster.

## Synchronization of Module Catalog with Remote Clusters

Lifecycle Manager ensures the Module Catalog is correctly synchronized with users' runtimes.
The Module Catalog consists of all modules, represented by ModuleTemplate CRs and related ModuleReleaseMetas CRs, that are available for a user. The Module Catalog portfolio may vary for different users.
The synchronization mechanism described below is essential to allow users to enable modules in their clusters.
The mechanism is controlled by the set of labels that are configured on Kyma and ModuleTemplate CRs in the Control Plane. The labels are: `operator.kyma-project.io/sync`, `operator.kyma-project.io/internal`, and `operator.kyma-project.io/beta`.
The v1beta2 API introduces three groups of modules:

* Default modules, synchronized by default.
* Internal modules, synchronized per-cluster only if configured explicitly on the corresponding Kyma CRs. To mark a ModuleTemplate CR as `internal`, use the `operator.kyma-project.io/internal` label and set it to `true`.
* Beta modules, synchronized per-cluster only if configured explicitly on the corresponding Kyma CRs. To mark a ModuleTemplate CR as `beta`, use the `operator.kyma-project.io/beta` label and set it to `true`.

By default, without any labels configured on Kyma and ModuleTemplate CRs, a ModuleTemplate CR is synchronized with remote clusters.
For every synchronized ModuleTemplate CR, all related ModuleReleaseMetas CRs are synchronized as well.

> [!Note]
> The ModuleTemplate CRs synchronization is enabled only when Lifecycle Manager runs in the control-plane mode. Lifecycle Manager running in the single-cluster mode doesn't require any CR synchronization.

> [!Note]
> Disabling synchronization for already synchronized ModuleTemplates CRs doesn't remove them from remote clusters. The CRs remain as they are, but any subsequent changes to these ModuleTemplate CRs in the Control Plane are not synchronized.

For more information, see [the Kyma CR synchronization labels](./resources/01-kyma.md#operatorkyma-projectio-labels).

## Kyma CR Synchronization

The Kyma CR serves as the main configuration file shared between KCP and SKR clusters. It contains crucial information including the list of modules to be enabled in the **.spec.modules** field. The synchronization process follows this strategy:

1. When an SKR is provisioned, the Kyma CR is first created on the KCP side. At this point, it can contain a list of default modules in the **.spec.modules** field.

2. Lifecycle Manager then synchronizes this CR, along with its Custom Resource Definition (CRD), to the SKR cluster.

3. Users can modify the Kyma CR in the SKR cluster either:
    - Manually by editing the CR directly
    - Through the SAP BTP cockpit user interface

4. Users' modifications may include:
    - Adding or removing modules
    - Changing module versions
    - Updating module configurations

5. Lifecycle Manager monitors these changes by reading the Kyma CR in the SKR cluster, which contains the updated module list.

> [!Note]
> The **.spec.modules** field in the Kyma CR on the KCP side remains unchanged and only contains the default modules. The actual module configuration is maintained in the SKR's Kyma CR.

For more details about the Kyma CR structure and fields, see the [Kyma CR documentation](./resources/01-kyma.md).
