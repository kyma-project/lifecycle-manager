# Lifecycle Manager - Operations

## Metrics

Lifecycle Manager metrics are exposed over port 8080 through the metrics endpoint "/metrics".

The following metrics are exposed:

| Metric Name                              | Metric Type    | Description                                                                                  |
|------------------------------------------|----------------|----------------------------------------------------------------------------------------------|
| `lifecycle_mgr_requeue_reason_total`     | Counter Vector | Indicates the requeue reason of a reconciler.                                                |
| `lifecycle_mgr_kyma_state`               | Gauge Vector   | Indicates the state of a Kyma CR.                                                            |
| `lifecycle_mgr_module_state`             | Gauge Vector   | Indicates the state of a module enabled on a Kyma CR.                                        |
| `lifecycle_mgr_mandatory_modules`        | Gauge          | Indicates the number of mandatory ModuleTemplates.                                           |
| `lifecycle_mgr_mandatory_module_state`   | Gauge Vector   | Indicates the state of a mandatory module enabled on a Kyma CR.                              |
| `reconcile_duration_seconds`             | Gauge Vector   | Indicates the duration of a manifest reconciliation in seconds.                              |
| `lifecycle_mgr_purgectrl_time`           | Gauge          | Indicates the average duration of purge reconciliation.                                      |
| `lifecycle_mgr_purgectrl_requests_total` | Counter        | Indicates the total number of purges.                                                        |
| `lifecycle_mgr_purgectrl_error`          | Gauge Vector   | Indicates the errors produced by purge.                                                      |
| `lifecycle_mgr_self_signed_cert_not_renew` | Gauge Vector  | Indicates that the self-signed Certificate of a Kyma CR is not renewed yet.                 |


## Dashboards
The above-mentioned metrics are visualized using Grafana and grouped into four dashboards:

### 1. Lifecycle Manager Overview
This dashboard gives an overview of the Lifecycle Manager. It includes the following:

| Panel Name                                | Description                                                                                                                                          |
|-------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| Lifecycle Manager Kube API Request Rate   | Shows the rate of requests to the Kubernetes API server.                                                                                             |
| Lifecycle Manager Memory                  | Shows the memory usage of the Lifecycle Manager.                                                                                                     |
| Lifecycle Manager CPU Usage               | Shows the CPU usage of the Lifecycle Manager.                                                                                                        |
| Lifecycle Manager Controller Runtime Reconcile Success | Time series showing the success rate of all the reconcilers.                                                                                         |
| Operator Controller Runtime Reconcile Errors | Time series showing the errors produced by all the reconcilers.                                                                                      |
| Kyma Requeue Reason                       | Shows the requeue reasons of all the reconcilers controlling the different CRs.                                                                      |
| Reconcile Duration                        | Shows the duration of the Kyma and manifest reconciliations.                                                                                         |
| Max Workers                               | Shows the maximum number of workers used by the Kyma and Manifest reconcilers.                                                                       |
| Active Workers                            | Shows the number of active workers used by the Kyma and Manifest reconcilers.                                                                        |
| Workqueue Longest Running Processor Seconds | Indicates how many seconds the longest running processor for workqueue has been running.                                                             |
| Workqueue Unfinished Work Seconds         | Indicates how many seconds of work has been done that is in progress and hasn’t been observed by work_duration. Large values indicate stuck threads. |
| Work Queue Processing Latency             | Indicates how long in seconds an item stays in workqueue before being requested.                                                                     |
| Work Queue Processing Duration            | Indicates how long in seconds processing an item from workqueue takes.                                                                               |
| Work Queue Depth                          | Indicates the number of actions waiting in the queue to be performed.                                                                                |
| Work Queue Add Rate                       | Indicates the rate of adding actions to the queue.                                                                                                   |
| Self-signed Certificate Not Renew         | Indicates the self-signed Certificate of related Kyma is not renewed yet.                                                                            |
| Purge Duration Seconds                    | Shows the duration of the purge reconciliation.                                                                                                      |
| Purge Count                               | Shows the total number of purges.                                                                                                                    |
| Purge Errors                              | Shows the errors produced by the purge.                                                                                                              |


### 2. Lifecycle Manager Kyma Status
This dashboard gives an overview of the Kyma CRs and modules states. It includes the following:

| Panel Name                    | Description                                                                                     |
|-------------------------------|-------------------------------------------------------------------------------------------------|
| Enabled Modules               | Shows the statistics of the enabled modules on the SKR Clusters, including the total number of each module. |
| Kyma Manifest in Error State  | Shows the manifests in Error state with information about the Kyma name, module name, and shoot name. |
| Kyma State Total              | Graph showing the number of Kyma CRs in each state over time.                                  |
| Module State Total            | Graph showing the number of modules enabled on Kyma CRs in each state over time.               |
| Mandatory Module State Total  | Graph showing the number of mandatory modules enabled on Kyma CRs in each state over time.     |
| Mandatory Modules Count       | Indicates the number of mandatory modules installed on the SKR clusters.                       |


### 3. Lifecycle Manager Watcher Components
This dashboard gives an overview of the Watcher Components. It includes the following:

| Panel Name                      | Description                                                                                                   |
|---------------------------------|---------------------------------------------------------------------------------------------------------------|
| Usage                           | Shows the statistics of Watcher deployments installed on the SKR Cluster, including the total number of SKRs, the percentage of SKRs with Watcher deployment, and the total number of Watcher deployments. |
| Images on Shoots                | A table mapping the Watcher images installed on SKR Clusters with their numbers.                              |
| Unready SKR Watcher Deployments | Shows the total number of unready Watcher deployments on the SKR Clusters.                                    |
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


### 4. Lifecycle Manager Mandatory Modules

This dashboard gives an overview of all the mandatory modules installed on the SKR Clusters. It includes the following metrics:

| Panel Name                   | Description                                                                                                   |
|------------------------------|---------------------------------------------------------------------------------------------------------------|
| Warden Usage                 | Shows the statistics of Warden module installation on the SKR Clusters, including the total number of SKRs, the percentage of SKRs with Warden deployment, and the total number of Warden deployments. |
| Warden Images on Shoots      | A table mapping the Warden images installed on SKR Clusters with their numbers.                              |
| Unready Warden Deployments   | Shows the total number of unready Warden deployments on the SKR Clusters.                                    |



The following Prometheus rules are in place to alert if some metrics are not in the expected state:

1. The lifecycle_mgr_self_signed_cert_not_renew metric has the value of 1 for 30 minutes, indicating that the Kyma self-signed certificate renewal buffer time has been exceeded by 30 minutes.
 