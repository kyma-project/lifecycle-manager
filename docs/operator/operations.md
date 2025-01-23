# Lifecycle Manager - Operations

## Metrics

Lifecycle Manager metrics are exposed over port 8080 through the metrics endpoint "/metrics".

The following metrics are exposed:

### General metrics:
#### lifecycle_mgr_requeue_reason_total
This metric is a counter vector that indicates the requeue reason of a reconciler.

### Kyma related metrics:
#### lifecycle_mgr_kyma_state
This metric is a gauge vector that indicates the state of a Kyma CR.
#### lifecycle_mgr_module_state
This metric is a gauge vector that indicates the state of a module enabled on a Kyma CR.

### Mandatory Modules related metrics:
#### lifecycle_mgr_mandatory_modules
This metric is a gauge that indicates the number of mandatory ModuleTemplates.
#### lifecycle_mgr_mandatory_module_state
This metric is a gauge vector that indicates the state of a mandatory module enabled on a Kyma CR.

### Manifest related metrics:
#### reconcile_duration_seconds
This metric is a gauge vector that indicates the duration of a manifest reconciliation in seconds.

### Purge related metrics:
#### lifecycle_mgr_purgectrl_time
This metric is a gauge that indices the average duration of purge reconciliation.
#### lifecycle_mgr_purgectrl_requests_total
This metric is a counter that indicates the total number of purges.
#### lifecycle_mgr_purgectrl_error
This metric is a gauge vector that indicates the errors produced by purge.

### Watcher related metrics:
#### lifecycle_mgr_self_signed_cert_not_renew
This metric is a gauge vector that indicates that the self-signed Certificate of a Kyma CR is not renewed yet.

## Dashboards
The above-mentioned metrics are visualized using Grafana and grouped into four dashboards:

### 1. Lifecycle Manager Overview
This dashboard gives an overview of the Lifecycle Manager. It includes the following:

#### Lifecycle Manager Kube API Request Rate
This panel shows the rate of requests to the Kubernetes API server.

#### Lifecycle Manager Memory
This panel shows the memory usage of the Lifecycle Manager.

#### Lifecycle Manager CPU Usage
This panel shows the CPU usage of the Lifecycle Manager.

#### Lifecycle Manager Controller Runtime Reconcile Success
This panel is time series that shows the success rate of all the reconcilers.

#### Operator Controller Runtime Reconcile Errors
This panel is a time series that shows the errors produced by all the reconcilers.

#### Kyma Requeue Reason
This panel shows the requeue reasons of all the reconcilers controlling the different CRs.

#### Reconcile Duration
This panel shows the duration of the kyma and manifest reconciliations.

#### Max Workers
This panel shows the maximum number of workers used by the Kyma and Manifest reconcilers.

#### Active workers
This panel shows the number of active workers used by the Kyma and Manifest reconcilers.

#### Workqueue Longest running processor seconds
This panel indicates how many seconds has the longest running processor for workqueue been running.

#### Workqueue Unfinished Work Seconds
This panel indicates how many seconds of work has been done that is in progress and hasn’t been observed by work_duration. Large values indicate stuck threads.

#### Work Queue Processing Latency
This panel indicates how long in seconds an item stays in workqueue before being requested.

#### Work Queue Processing Duration
This panel indicates how long in seconds processing an item from workqueue takes.

#### Work Queue Depth
This panel indicates the number of actions waiting in the queue to be performed.

#### Work Queue Add Rate
This panel indicates the rate of adding actions to the queue.

#### Self-signed Certificate Not Renew
This panel indicates the self-signed Certificate of related Kyma is not renewed yet.

#### Purge Duration Seconds
This panel shows the duration of the purge reconciliation.

#### Purge Count
This panel shows the total number of purges.

#### Purge Errors
This panel shows the errors produced by the purge.

### 2. Lifecycle Manager Kyma Status
This dashboard gives an overview of the Kyma CRs and modules states. It includes the following:

#### Enabled Modules
This panel shows the statistics of the enabled modules on the SKR Clusters. It shows the total number of each module.

#### Kyma Manifest in Error State
This panel shows the manifests that are in Error state with information about the kyma name, module name and shoot name.

#### Kyma State Total
This panel is a graph showing the number of Kyma CRs in each state over the time.

#### Module State Total
This panel is a graph showing the number of modules enabled on Kyma CRs in each state over the time.

#### Mandatory Module State Total
This panel is a graph showing the number of mandatory modules enabled on Kyma CRs in each state over the time.

#### Mandatory Modules Count
This panel indicates the number of mandatory modules installed on the SKR clusters.

### 3. Lifecycle Manager Watcher Components
This dashboard gives an overview of the Watcher Components. It includes the following:

#### Usage
This panel shows the statistics of Watcher deployments installed on the SKR Cluster. It shows the total number of SKRs, the percentage of SKRs
that have Watcher deployment and the total number of Watcher deployments throughout the SKRs.

#### Images on Shoots
This panel is a table mapping the Watcher images installed on SKR Clusters with their numbers.

#### Unready SKR Watcher Deployments
This panel shows the total number of unready Watcher deployments on the SKR Clusters.

#### Requests per Minute
This panel is a timeseries which shows the rate of requests per minute to the Listener.

#### Request Duration
This panel is a graph which shows the duration of requests to the Listener.

#### Pending Requests
This panel shows the total number of pending requests to the Listener.

#### Failed Requests per Minute
This panel is a timeseries which shows the rate of failed requests per minute to the Listener. It shows the URI for the failed requests.

#### AdmissionRequest Duration
This panel is a graph which shows the duration of AdmissionRequests. It shows the error reason for the failed AdmissionRequests.

#### AdmissionRequest Error Total
This panel shows the total number of errors in AdmissionRequests.

#### AdmissionRequests Total
This panel shows the total number of AdmissionRequests.

#### Failed KCP Requests Total
This panel shows the total number of failed KCP requests.

#### KCP Requests Total
This panel shows the total number of KCP requests.

#### Requests Ratio
This panel shows the ratio of AdmissionRequests to the total KCP requests.

### 4. Lifecycle Manager Mandatory Modules

This panel gives an overview of all the mandatory modules installed on the SKR Clusters. It includes the following metrics:

#### Warden Usage
This panel shows the statistics of Warden module installation on the SKR Clusters. It shows the total number of SKRs, the percentage of SKRs
that have Warden deployment and the total number of Warden deployments throughout the SKRs. 

#### Warden Images on Shoots
This panel is a table mapping the Warden images installed on SKR Clusters with their numbers.

#### Unready Warden Deployments
This panel shows the total number of unready Warden deployments on the SKR Clusters.


Some prometheus rules are in place to alert if some metrics are not in the expected state:

1. The lifecycle_mgr_self_signed_cert_not_renew metric has the value of 1 for 30 minutes, indicating that the Kyma self-signed certificate renewal buffer time has been exceeded by 30 minutes.
 