# Lifecycle Manager - Operations

## Metrics

Lifecycle Manager metrics are exposed over port 8080 through the metrics endpoint "/metrics".

The metrics are grouped into four dashboards:

### 1. Lifecycle Manager Overview

### 2. Lifecycle Manager Kyma Status



### 3. Lifecycle Manager Watcher Components

This dashboard gives an overview of the Watcher Components. It includes the following:

#### Usage
This metric shows the statistics of Watcher deployments installed on the SKR Cluster. It shows the total number of SKRs, the percentage of SKRs
that have Watcher deployment and the total number of Watcher deployments throughout the SKRs.

#### Images on Shoots
This metric is a table mapping the Watcher images installed on SKR Clusters with their numbers.

#### Unready SKR Watcher Deployments
This metric shows the total number of unready Watcher deployments on the SKR Clusters.

#### Requests per Minute
This metric is a timeseries which shows the rate of requests per minute to the Listener.

#### Request Duration
This metric is a graph which shows the duration of requests to the Listener.

#### Pending Requests
This metric shows the total number of pending requests to the Listener.

#### Failed Requests per Minute
This metric is a timeseries which shows the rate of failed requests per minute to the Listener. It shows the URI for the failed requests.

#### AdmissionRequest Duration
This metric is a graph which shows the duration of AdmissionRequests. It shows the error reason for the failed AdmissionRequests.

#### AdmissionRequest Error Total
This metric shows the total number of errors in AdmissionRequests.

#### AdmissionRequests Total
This metric shows the total number of AdmissionRequests.

#### Failed KCP Requests Total
This metric shows the total number of failed KCP requests.

#### KCP Requests Total
This metric shows the total number of KCP requests.

#### Requests Ratio
This metric shows the ratio of AdmissionRequests to the total KCP requests.

### 4. Lifecycle Manager Mandatory Modules

This dashboard gives an overview of all the mandatory modules installed on the SKR Clusters. It includes the following metrics:

#### Warden Usage
This metric shows the statistics of Warden module installation on the SKR Clusters. It shows the total number of SKRs, the percentage of SKRs
that have Warden deployment and the total number of Warden deployments throughout the SKRs. 

#### Warden Images on Shoots
This metric is a table mapping the Warden images installed on SKR Clusters with their numbers.

#### Unready Warden Deployments
This metric shows the total number of unready Warden deployments on the SKR Clusters.

