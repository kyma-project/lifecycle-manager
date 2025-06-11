# Lifecycle Manager Components

The following table shows the Kubernetes components that need to exist in the Control-Plane Cluster to have the Lifecycle Manager up and running.

| Kind                                | Name                                               | Namespace     | Description |
|-------------------------------------|----------------------------------------------------|---------------|-------------|
| `CustomResourceDefinition`          | `kymas.operator.kyma-project.io`                   | Cluster-wide  | CRD for managing Kyma custom resources. |
| `CustomResourceDefinition`          | `manifests.operator.kyma-project.io`               | Cluster-wide  | CRD for deployment and image configurations. |
| `CustomResourceDefinition`          | `moduletemplates.operator.kyma-project.io`         | Cluster-wide  | CRD template for defining reusable module templates. |
| `CustomResourceDefinition`          | `modulereleasemetas.operator.kyma-project.io`      | Cluster-wide  | CRD for managing module release channels and versions. |
| `CustomResourceDefinition`          | `watchers.operator.kyma-project.io`                | Cluster-wide  | CRD for watching and reacting to changes in resources. |
| `ServiceAccount`                    | `klm-controller-manager`                           | `kcp-system`  | Main controller's service account. |
| `ClusterRole`                       | `klm-controller-manager-crds`                      | Cluster-wide  | Grants permissions to manage CRDs. |
| `ClusterRole`                       | `klm-controller-manager-role`                      | Cluster-wide  | Grants access to Kyma and module CRDs. |
| `ClusterRole`                       | `klm-controller-manager-metrics-reader`            | Cluster-wide  | Grants permissions to read controller metrics. |
| `ClusterRoleBinding`               | `klm-controller-manager-rolebinding`              | Cluster-wide  | Binds controller role to its service account. |
| `ClusterRoleBinding`               | `klm-controller-manager-metrics-reader`           | Cluster-wide  | Binds metrics reader role to the service account. |
| `Role`                              | `klm-controller-manager-leader-election`           | `kcp-system`  | Used for leader election coordination. |
| `Role`                              | `klm-controller-manager`                           | `kcp-system`  | Role for accessing runtime resources. |
| `Role`                              | `klm-controller-manager-certmanager`               | `istio-system`| Role for cert-manager integration. |
| `RoleBinding`                       | `klm-controller-manager-leader-election`           | `kcp-system`  | Binds leader election role to service account. |
| `RoleBinding`                       | `klm-controller-manager`                           | `kcp-system`  | Binds runtime role to service account. |
| `RoleBinding`                       | `klm-controller-manager-certmanager`               | `istio-system`| Binds cert-manager role to service account. |
| `Service`                           | `klm-controller-manager-events`                    | `kcp-system`  | Exposes controller metrics/events ports. |
| `Service`                           | `lifecycle-manager`                                | `kcp-system`  | Main service endpoint for Lifecycle Manager. |
| `Deployment`                        | `klm-controller-manager`                           | `kcp-system`  | Main controller logic for managing Kyma modules. |
| `ConfigMap`                         | `klm-dashboard-overview`                           | `kcp-system`  | Grafana dashboard config for overview panel. |
| `ConfigMap`                         | `klm-dashboard-watcher`                            | `kcp-system`  | Grafana dashboard config for watcher view. |




