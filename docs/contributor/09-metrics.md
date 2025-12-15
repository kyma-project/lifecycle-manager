# Lifecycle Manager Metrics

Lifecycle Manager metrics are exposed over port `8080` through the metrics endpoint `/metrics`.

The following metrics are exposed:

| Metric Name                              | Metric Type    | Metric Labels                                                 | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
|------------------------------------------|----------------|---------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `lifecycle_mgr_requeue_reason_total`     | Counter Vector | `requeue_reason`<br/>`requeue_type`                               | Indicates the requeue reason of the Lifecycle Manager reconcilers. See [Controllers](02-controllers.md).                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `lifecycle_mgr_kyma_state`               | Gauge Vector   | `kyma_name`<br/>`state`<br/>`shoot`<br/>`instance_id`                 | Indicates the state of a Kyma CR. The state can be one of the following:<ul><li>`Error`: An error is blocking the synchronization of the Kyma CR with the SKR cluster.</li><li>`Ready`: The Kyma CR is synchronized with the SKR cluster.</li><li>`Processing`: The Kyma CR is being synchronized with the SKR cluster.</li><li>`Warning`: Some misconfiguration, that requires the user's action, is blocking the Kyma CR synchronization with the SKR cluster. </li><li>`Deleting`: The Kyma CR and its modules are being removed from the SKR cluster.</li></ul>        |
| `lifecycle_mgr_module_state`             | Gauge Vector   | `module_name`<br/>`kyma_name`<br/>`state`<br/>`shoot`<br/>`instance_id` | Indicates the state of a module added to a Kyma CR. The state can be one of the following:<ul><li>`Error`: An error is blocking the installation of the module in the SKR cluster. </li><li>`Ready`: The module is successfully installed in the SKR cluster. </li><li>`Processing`: The module is still being installed in the SKR cluster. </li><li>`Warning`: Some misconfiguration, that requires the user's action, is blocking the module installation in the SKR cluster.</li><li>`Deleting`: The module resources are still being removed from the SKR cluster.</li></ul> |
| `lifecycle_mgr_mandatory_modules`        | Gauge          |                                                               | Indicates the number of mandatory ModuleTemplate CRs.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `lifecycle_mgr_mandatory_module_state`   | Gauge Vector   | `module_name`<br/>`kyma_name`<br/>`state`                           | Indicates the state of a mandatory module added to a Kyma CR. The state value can be one of the following:  `Error`, `Ready`, `Processing`, `Warning`, or `Deleting`.                                                                                                                                                                                                                                                                                                                                                                                                   |
| `reconcile_duration_seconds`             | Gauge Vector   | `manifest_name`                                                 | Indicates the duration of a Manifest CR reconciliation in seconds.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `lifecycle_mgr_purgectrl_time`           | Gauge          |                                                               | Indicates the average duration of purge reconciliation. See [Purge Controller](02-controllers.md#purge-controller).                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `lifecycle_mgr_purgectrl_requests_total` | Counter        |                                                               | Indicates the total number of purges.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `lifecycle_mgr_purgectrl_error`          | Gauge Vector   | `kyma_name`<br/>`instance_id`<br/>`shoot`<br/>`err_reason`            | Indicates the errors produced by the purge.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `lifecycle_mgr_self_signed_cert_not_renew` | Gauge Vector  | `kyma_name`                                                     | Indicates that the self-signed Certificate of a Kyma CR is not renewed yet. This metric is just to verify that the renewal of the certificate is working as expected since we rely on the cert-manager mechanism for the certificate rotation.                                                                                                                                                                                                                                                                                                                          |
| `lifecycle_mgr_maintenance_window_config_read_success`    | Gauge          |                                                               | Indicates whether the maintenance window configuration was read successfully.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |

The metrics are grouped by the following labels:

* `requeue_reason`: Indicates the reason for the Lifecycle Manager reconcilers' requeue. See [Controllers](02-controllers.md). The reasons describe a specific error and/or the reason for the synchronization between the KCP and SKR clusters.
* `requeue_type`: Indicates whether the requeue is expected or not. The possible values are `intended` and `unexpected`.
* `kyma_name`: The Kyma CR name.
* `state`: The state of the module, Manifest or Kyma CRs. The possible values are `Error`, `Ready`, `Processing`, `Warning`, and `Deleting`.
* `shoot`: The name of the SKR cluster.
* `instance_id`: The instance id.
* `module_name`: The module name.
* `err_reason`: The error reason for the purge reconciler. The possible values are `PurgeFinalizerRemovalError` and `CleanupError`.
* `manifest_name`: The name of the Manifest CR.

## Dashboards

The above-mentioned metrics are visualized using Grafana and grouped into four dashboards:

### 1. Lifecycle Manager Overview

This dashboard gives an overview of Lifecycle Manager. It includes the following:

| Panel Name                                             | Description                                                                                                                                          |
|--------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| Lifecycle Manager Kube API Request Rate                | Shows the rate of requests to the Kubernetes API server.                                                                                             |
| Lifecycle Manager Memory                               | Shows the memory usage of Lifecycle Manager.                                                                                                     |
| Lifecycle Manager CPU Usage                            | Shows the CPU usage of Lifecycle Manager.                                                                                                        |
| Lifecycle Manager Controller Runtime Reconcile Success | Time series showing the success rate of all the reconcilers.                                                                                         |
| Operator Controller Runtime Reconcile Errors           | Time series showing the errors produced by all the reconcilers.                                                                                      |
| Kyma Requeue Reason                                    | Shows the requeue reasons of all the reconcilers controlling the different CRs.                                                                      |
| Reconcile Duration                                     | Shows the duration of the Kyma and Manifest CRs reconciliations.                                                                                         |
| Max Workers                                            | Shows the maximum number of workers that Kyma and Manifest CRs reconcilers use.                                                                       |
| Active Workers                                         | Shows the number of active workers that Kyma and Manifest CRs reconcilers use.                                                                        |
| Workqueue Longest Running Processor Seconds            | Indicates how long the longest running processor for workqueue was running in seconds.                                                             |
| Workqueue Unfinished Work Seconds                      | Indicates how many seconds of work has been done that is in progress and hasn’t been observed by work_duration. Large values indicate stuck threads. |
| Work Queue Processing Latency                          | Indicates how long in seconds an item stays in workqueue before being requested.                                                                     |
| Work Queue Processing Duration                         | Indicates how long in seconds processing an item from workqueue takes.                                                                               |
| Work Queue Depth                                       | Indicates the number of actions waiting in the queue to be performed.                                                                                |
| Work Queue Add Rate                                    | Indicates the rate of adding actions to the queue.                                                                                                   |
| Self-signed Certificate Not Renew                      | Indicates the self-signed Certificate of the related Kyma CR has not been renewed yet.                                                                            |
| Purge Duration Seconds                                 | Shows the duration of the purge reconciliation.                                                                                                      |
| Purge Count                                            | Shows the total number of purges.                                                                                                                    |
| Purge Errors                                           | Shows the errors produced by the purge.                                                                                                              |
| Maintenance Window Configuration Read Status           | Indicates whether the maintenance window configuration was read successfully.                                                                        |

### 2. Kyma CRs Status

This dashboard gives an overview of the Kyma CRs and modules states. It includes the following:

| Panel Name                    | Description                                                                                     |
|-------------------------------|-------------------------------------------------------------------------------------------------|
| Enabled Modules               | Shows the number of all modules enabled in the SKR Clusters, including the total number for every module. |
| Kyma Manifest in Error State  | Shows the Manifest CRs in the `Error` state with the information about the relevant Kyma CR name, module name, and shoot name. |
| Kyma State Total              | A graph showing the number of Kyma CRs in each state over time.                                  |
| Module State Total            | A graph showing the number of modules added to Kyma CRs in each state over time.               |
| Mandatory Module State Total  | A graph showing the number of mandatory modules added to Kyma CRs in each state over time.     |
| Mandatory Modules Count       | Indicates the number of mandatory modules installed in the SKR clusters.                       |

### 3. Runtime Watcher Components

This dashboard gives an overview of the [Runtime Watcher](https://github.com/kyma-project/runtime-watcher/tree/main) components. It includes the following:

| Panel Name                      | Description                                                                                                   |
|---------------------------------|---------------------------------------------------------------------------------------------------------------|
| Usage                           | Shows the statistics of Runtime Watcher deployments installed in the SKR cluster, including the total number of SKRs, the percentage of SKRs with a Runtime Watcher deployment, and the total number of Runtime Watcher deployments. |
| Images on Shoots                | A table mapping Runtime Watcher images installed in SKR clusters with their numbers.                              |
| Unready SKR Watcher Deployments | Shows the total number of unready Runtime Watcher deployments in the SKR clusters.                                    |
| Requests per Minute             | A time series showing the rate of requests per minute to the Listener.                                        |
| Request Duration                | A graph showing the duration of requests to the Listener.                                                     |
| Pending Requests                | Shows the total number of pending requests to the Listener.                                                   |
| Failed Requests per Minute      | A time series showing the rate of failed requests per minute to the Listener, including the URI for failed requests. |
| AdmissionRequest Duration       | A graph showing the duration of AdmissionRequests, including the error reasons for failed requests.           |
| AdmissionRequest Error Total    | Shows the total number of errors in AdmissionRequests.                                                        |
| AdmissionRequests Total         | Shows the total number of AdmissionRequests.                                                                  |
| Failed KCP Requests Total       | Shows the total number of failed KCP requests.                                                                |
| KCP Requests Total              | Shows the total number of KCP requests.                                                                       |
| Requests Ratio                  | Shows the ratio of AdmissionRequests to the total KCP requests.                                               |

### 4. Mandatory Modules

This dashboard gives an overview of all the mandatory modules installed in the SKR clusters. It includes the following metrics:

| Panel Name                   | Description                                                                                                   |
|------------------------------|---------------------------------------------------------------------------------------------------------------|
| Warden Usage                 | Shows the statistics of the Warden module installation in the SKR clusters, including the total number of SKRs, the percentage of SKRs with a Warden deployment, and the total number of Warden deployments. |
| Warden Images on Shoots      | A table mapping the Warden images installed in SKR clusters with their numbers.                              |
| Unready Warden Deployments   | Shows the total number of unready Warden deployments in the SKR clusters.                                    |

## Prometheus Rules

The following Prometheus rules are in place to alert if some metrics are not in the expected state:

* The `lifecycle_mgr_self_signed_cert_not_renew` metric has the value of `1` for 30 minutes, indicating that the Kyma self-signed certificate renewal buffer time has been exceeded by 30 minutes.
