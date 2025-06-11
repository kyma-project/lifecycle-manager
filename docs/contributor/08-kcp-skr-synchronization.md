# Synchronization Between Kyma Control Plane and SAP BTP, Kyma Runtime

This document explains how the Kyma Control Plane (KCP) cluster, where Lifecycle Manager operates, synchronizes with the user's runtime cluster, namely the SAP BTP, Kyma runtime (SKR) cluster.

## Module Catalog Synchronization

The Module Catalog comprises all modules, represented by ModuleTemplate and related ModuleReleaseMeta custom resources (CRs), that are available for SAP BTP, Kyma runtime users. The Module Catalog portfolio may vary for different user groups.

Lifecycle Manager ensures the Module Catalog is correctly synchronized with users' runtimes. The synchronization mechanism allows users to enable modules in their clusters. The mechanism is controlled by the set of labels that are configured on Kyma and ModuleTemplate CRs in the KCP. The labels are:
* `operator.kyma-project.io/internal`
* `operator.kyma-project.io/beta`

The v1beta2 API introduces three groups of modules:

* **Default** modules that are synchronized by default.
* **Internal** modules that are synchronized per cluster only if the corresponding Kyma CR has the `operator.kyma-project.io/internal` label. A ModuleTemplate CR is `internal` if the module team sets the `operator.kyma-project.io/internal` label to `true`.
* **Beta** modules that are synchronized per cluster only if the corresponding Kyma CR contains the `operator.kyma-project.io/beta` label. A ModuleTemplate CR is `beta` if the module team sets the `operator.kyma-project.io/beta` label to `true`.

By default, without any labels configured on Kyma and ModuleTemplate CRs, a ModuleTemplate CR is synchronized with SAP BTP, Kyma runtime clusters. For every synchronized ModuleTemplate CR, all related ModuleReleaseMeta CRs are synchronized as well.

> [!Note]
> Disabling synchronization for already synchronized ModuleTemplates CRs doesn't remove them from the SAP BTP, Kyma runtime clusters. The CRs remain as they are, but any subsequent changes to these ModuleTemplate CRs in the Kyma Control Plane are not synchronized.

For more information, see [`operator.kyma-project.io` Labels](./resources/01-kyma.md#operatorkyma-projectio-labels).

## Kyma CR Synchronization

The Kyma CR serves as the main configuration file shared between KCP and SKR clusters. It contains crucial information. For example, the **.spec.modules** field includes the list of modules to be enabled in the SKR cluster. The Kyma CR synchronization process follows this strategy:

1. When an SKR cluster is provisioned, a Kyma CR is first created in the KCP cluster. The newly created Kyma CR contains a list of default modules in the **.spec.modules** field.

2. Lifecycle Manager synchronizes the Kyma CR, along with its Custom Resource Definition (CRD), to the SKR cluster. As a result, another Kyma CR is created in the SKR cluster.

3. SKR users can modify the Kyma CR in the SKR cluster either:
    - Manually using kubectl or Kyma CLI
    - In Kyma dashboard

4. Users' modifications may include:
    - Adding or removing modules
    - Changing module versions

5. Lifecycle Manager monitors the changes by reading the Kyma CR in the SKR, but does not synchronize the KCP Kyma CR.

> [!Note]
> The **.spec.modules** field in the KCP Kyma CR remains unchanged and contains only the default modules. The actual module configuration is maintained in the SKR Kyma CR.

For more details about the Kyma CR structure and fields, see [Kyma](./resources/01-kyma.md) CR documentation.
