# Lifecycle Manager Components

To run, Lifecycle Manager requires a set of Kubernetes components that must exist in the Kyma Control Plane (KCP) cluster. The following table lists and describes all the building blocks, specifying the namespace where each resides.

| Kind                       | Name                                          | Namespace      | Description                                                                                            |
|----------------------------|-----------------------------------------------|----------------|--------------------------------------------------------------------------------------------------------|
| `CustomResourceDefinition` | `kymas.operator.kyma-project.io`              | Cluster-wide   | Custom Resource Definition (CRD) for managing Kyma custom resources.                                   |
| `CustomResourceDefinition` | `manifests.operator.kyma-project.io`          | Cluster-wide   | CRD for module deployment and image configurations.                                                    |
| `CustomResourceDefinition` | `moduletemplates.operator.kyma-project.io`    | Cluster-wide   | CRD for defining module images and resources.                                                          |
| `CustomResourceDefinition` | `modulereleasemetas.operator.kyma-project.io` | Cluster-wide   | CRD for mapping module versions to corresponding channels.                                             |
| `CustomResourceDefinition` | `watchers.operator.kyma-project.io`           | Cluster-wide   | CRD for watching changes on specified resources in the SAP BTP, Kyma runtime (SKR) clusters.           |
| `Deployment`               | `klm-controller-manager`                      | `kcp-system`   | Main controller logic for managing all Kyma resources.                                                 |
| `ServiceAccount`           | `klm-controller-manager`                      | `kcp-system`   | Main controller's service account.                                                                     |
| `ClusterRole`              | `klm-controller-manager-crds`                 | Cluster-wide   | Grants permissions to manage CRDs.                                                                     |
| `ClusterRoleBinding`       | `klm-controller-manager-crds`                 | Cluster-wide   | Binds crds role to its service account.                                                                |
| `Role`                     | `klm-controller-manager-leader-election`      | `kcp-system`   | Grants permission for leader election.                                                                 |
| `RoleBinding`              | `klm-controller-manager-leader-election`      | `kcp-system`   | Binds leader election role to service account.                                                         |
| `Role`                     | `klm-controller-manager`                      | `kcp-system`   | Role for accessing runtime resources.                                                                  |
| `RoleBinding`              | `klm-controller-manager`                      | `kcp-system`   | Binds manager role to service account.                                                                 |
| `Role`                     | `klm-controller-manager-certmanager`          | `istio-system` | Role for cert-manager integration.                                                                     |
| `RoleBinding`              | `klm-controller-manager-certmanager`          | `istio-system` | Binds cert-manager role to service account.                                                            |
| `Service`                  | `klm-controller-manager-metrics`              | `kcp-system`   | Exposes controller metrics.                                                                            |
| `Certificate`              | `klm-watcher-serving`                         | `istio-system` | Self-signed watcher CA certificate.                                                                       |
| `Certificate`              | `klm-controller-manager-webhook-serving`      | `kcp-system`   | Lifecycle manager webhook certificate.                                                                 |
| `Issuer`                   | `klm-watcher-root`                            | `istio-system` | Issues the self-signed watcher certificates.                                                           |
| `Issuer`                   | `klm-controller-manager-selfsigned`           | `kcp-system`   | Issues the webhook serving certificates.                                                               |
| `Authorization Policy`     | `klm-controller-manager`                      | `kcp-system`   | Policy to allow access to metrics and webhooks.                                                        |

Additionally, deploy the following `Config Maps to expose metrics on a Grafana dashboard:

| Kind                       | Name                                          | Namespace      | Description                                                                                            |
|----------------------------|-----------------------------------------------|----------------|--------------------------------------------------------------------------------------------------------|
| `ConfigMap`                | `klm-dashboard-overview`                      | `kcp-system`   | Grafana dashboard config for overview panel.                                                           |
| `ConfigMap`                | `klm-dashboard-watcher`                       | `kcp-system`   | Grafana dashboard config for watcher view.                                                             |
| `ConfigMap`                | `klm-dashboard-mandatory-modules`             | `kcp-system`   | Grafana dashboard config for mandatory modules view.                                                   |
| `ConfigMap`                | `klm-dashboard-status`                        | `kcp-system`   | Grafana dashboard config for modules status view.                                                      |

To enable the Watcher component, it requires the following resources:

| Kind                       | Name                                          | Namespace      | Description                                                                                            |
|----------------------------|-----------------------------------------------|----------------|--------------------------------------------------------------------------------------------------------|
| `Watcher`                  | `klm-watcher`                                 | `kcp-system`   | Watches the changes done to remote Kyma custom resource.                                               |
| `Service`                  | `klm-controller-manager-events`               | `kcp-system`   | Exposes controller events.                                                                             |
| `Gateway`                  | `klm-watcher `                                | `kcp-system`   | Istio gateway that exposes the watcher endpoint over HTTPS for secure communication with SKR clusters. |

To enable webhooks for Lifecycle Manager, the Kubernetes resource of type `Service` with the name of `klm-webhook-service` must be deployed in the `kcp-system` namespace.

