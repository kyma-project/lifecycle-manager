Feature: Manage Kyma lifecycle on a Kubernetes cluster

  Customers can order an SKR cluster or bring their own
  cluster to retrieve a managed Kyma runtime. The reconciler
  has to manage full lifecycle of a Kyma installation.

  Background:
    Given KCP cluster was created
    * customer cluster was created
    * reconciler namespace was created in KCP cluster
    * Kyma reconciler was installed in dedicated reconciler namespace

  Scenario: Kyma CR is created including one central and one decentral module in KCP cluster and module deployed in customer cluster
    When Kyma CR including one central and one decentral module was created in KCP cluster
    Then central module CR is created in KCP cluster
    * decentral module CR is created in customer cluster
    * manifest CR for decentral module us created in KCP cluster
    * decentral module is deployed in customer cluster

  Scenario: Kyma CR is deleted in KCP cluster and Kyma system is deleted in customer cluster
    Given Kyma CR including one central and one decentral module was created in KCP cluster
    When Kyma CR was deleted in KCP cluster
    Then module CR is delete in KCP cluster
    * manifest CR is deleted in KCP cluster
    * module CR is deleted in customer cluster
    * module is undeployed in customer cluster

  Scenario: Kyma CR is updated by adding a central module in KCP cluster
    Given Kyma CR created in KCP cluster
    When Kyma CR was updated by adding a central module in KCP cluster
    Then module CR is created in KCP cluster

  Scenario: Kyma CR is updated by deleting a central module in KCP cluster
    Given Kyma CR including one central module created in KCP cluster
    When Kyma CR was updated by deleting the central module in KCP cluster
    Then module CR is deleted in KCP cluster

  Scenario: Kyma CR is updated by adding a decentral module in customer cluster and decentral module is deployed in customer cluster
    Given Kyma CR was created in KCP cluster
    * Kyma CR was created in customer cluster
    When Kyma CR was updated by adding a decentral module in customer cluster
    Then Kyma CR is updated in conditions in KCP cluster
    * manifest CR for decentral module is created in KCP cluster
    * module CR is created in customer cluster
    * module is deployed in customer cluster

  Scenario: Kyma CR is updated by deleting a decentral module in customer cluster and decentral module is undeployed in customer cluster
    Given Kyma CR including one decentral module was created in KCP cluster
    * Kyma CR was created in customer cluster
    When Kyma CR was updated by deleting the decentral module in customer cluster
    Then Kyma CR is updated in conditions in KCP cluster
    * manifest CR for decentral module is deleted in KCP cluster
    * module CR is deleted in customer cluster
    * module is undeployed in customer cluster

  Scenario: Kyma CR is deleted in customer cluster and recovered from KCP cluster
    Given Kyma CR was created in KCP cluster
    * Kyma CR was created in customer cluster
    When Kyma CR was deleted in customer cluster
    Then Kyma CR is copied from KCP cluster in customer cluster

  Scenario: Kyma CR is update with invalid change in customer cluster
    Given Kyma CR was created in KCP cluster
    * Kyma CR was created in customer cluster
    When Kyma CR was updated with invalid change in customer cluster
    Then Kyma CR is updated in customer cluster
    * value of invalid fields are replaced with values from Kyma CR in KCP cluster
    * event is added to Kyma CR with notification about rejected field values