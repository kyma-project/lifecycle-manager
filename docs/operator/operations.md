# Lifecycle Manager - Operations

## Metrics

Lifecycle Manager metrics are exposed over port 8080 through the metrics endpoint "/metrics".

The metrics are grouped into four dashboards:

### 1. Lifecycle Manager Overview

### 2. Lifecycle Manager Kyma Status

### 3. Lifecycle Manager Watcher Components

### 4. Lifecycle Manager Mandatory Modules

This dashboard gives an overview of all the mandatory modules installed on the SKR Clusters. It includes the following metrics:

#### Warden Usage: 
This metric shows the statistics of Warden module installation on the SKR Clusters. It shows the total number of SKRs, the percentage of SKRs
that have Warden deployment and the total number of Warden Deployments throughout the SKRs. 

#### Warden Images on Shoots
This metric is a table mapping the Warden images installed on SKR Clusters with their numbers.

#### Unready Warden Deployments
This metric shows the total number of unready Warden deployments.

