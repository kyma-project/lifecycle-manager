# Lifecycle Manager Flags

This document provides a list of flags that can be set to control some specific aspects of Lifecycle Manager. The flags are set as arguments to the Lifecycle Manager Deployment. You can find the flags in the respective tables:

## Concurrent Reconciliation Values

| Flag                                                   | Type | Default Value | Description                                                                             |
|--------------------------------------------------------|------|---------------|-----------------------------------------------------------------------------------------|
| `max-concurrent-kyma-reconciles`                       | int  | 1             | Maximum number of concurrent Kyma CR reconciles which can be run                        |
| `max-concurrent-manifest-reconciles`                   | int  | 1             | Maximum number of concurrent Manifest CR reconciles which can be run                    |
| `max-concurrent-watcher-reconciles`                    | int  | 1             | Maximum number of concurrent Watcher CR reconciles which can be run                     |
| `max-concurrent-mandatory-modules-reconciles`          | int  | 1             | Maximum number of concurrent Mandatory Modules installation reconciles which can be run |
| `max-concurrent-mandatory-modules-deletion-reconciles` | int  | 1             | Maximum number of concurrent Mandatory Modules deletion reconciles which can be run     |

## Reconciliation Requeue Intervals

| Flag                                                 | Type     | Default Value | Description                                                                                                    |
|------------------------------------------------------|----------|---------------|----------------------------------------------------------------------------------------------------------------|
| `kyma-requeue-success-interval`                      | duration | 30s           | Duration after which a Kyma CR in the Ready state is enqueued for reconciliation                               |
| `kyma-requeue-error-interval`                        | duration | 2s            | Duration after which a Kyma CR in the Error state is enqueued for reconciliation                               |
| `kyma-requeue-warning-interval`                      | duration | 30s           | Duration after which a Kyma CR in the Warning state is enqueued for reconciliation                             |
| `kyma-requeue-busy-interval`                         | duration | 5s            | Duration after which a Kyma CR in the Processing state is enqueued for reconciliation                          |
| `mandatory-module-requeue-success-interval`          | duration | 30s           | Duration after which a Kyma CR in the Ready state is enqueued for mandatory module installation reconciliation |
| `manifest-requeue-success-interval`                  | duration | 30s           | Duration after which a Manifest CR in the Ready state is enqueued for reconciliation                           |
| `manifest-requeue-error-interval`                    | duration | 2s            | Duration after which a Manifest CR in the Error state is enqueued for reconciliation                           |
| `manifest-requeue-warning-interval`                  | duration | 30s           | Duration after which a Manifest CR in the Warning state is enqueued for reconciliation                         |
| `manifest-requeue-busy-interval`                     | duration | 5s            | Duration after which a Manifest CR in the Processing state is enqueued for reconciliation                      |
| `manifest-requeue-jitter-probability`                | float    | 0.02          | Percentage probability that jitter is applied to the requeue interval                                          |
| `manifest-requeue-jitter-percentage`                 | float    | 0.02          | Percentage range for the jitter applied to the requeue interval e.g. 0.1 means +/- 10% of the interval         |
| `mandatory-module-deletion-requeue-success-interval` | duration | 30s           | Duration after which a Kyma CR in the Ready state is enqueued for mandatory module deletion reconciliation     |
| `watcher-requeue-success-interval`                   | duration | 30s           | Duration after which a Watcher CR in the Ready state is enqueued for reconciliation                            |
| `istio-gateway-secret-requeue-success-interval`      | duration | 5m            | Duration after which the Istio Gateway Secret is enqueued after successful reconciliation                      |
| `istio-gateway-secret-requeue-error-interval`        | duration | 2s            | Duration after which the Istio Gateway Secret is enqueued after unsuccessful reconciliation                    |

## Controllers Configuration

| Flag                     | Type     | Default Value | Description                                                                                                                                                                                      |
|--------------------------|----------|---------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `rate-limiter-burst`     | int      | 200           | Maximum number of requests that can be processed immediately by all controllers before being rate-limited. Controls how many reconciliations can happen in quick succession during burst periods |
| `rate-limiter-frequency` | int      | 30            | Number of requests per second added to the rate limiter bucket for all controllers. Controls how many reconciliations are allowed to happen in steady state                                      |
| `failure-base-delay`     | duration | 100ms         | Duration of the failure base delay for rate limiting in all controllers                                                                                                                          |
| `failure-max-delay`      | duration | 5s            | Duration of the failure max delay for rate limiting in all controllers                                                                                                                           |
| `cache-sync-timeout`     | duration | 2m            | Duration of the cache sync timeout in all controllers                                                                                                                                            |

## Kubernetes Client Configuration

| Flag               | Type  | Default Value | Description                                                                                                                                                          |
|--------------------|-------|---------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `k8s-client-qps`   | int   | 1000           | Maximum queries per second (QPS) limit for the Kubernetes client. Controls how many requests can be made to the Kubernetes API server per second in the steady state |
| `k8s-client-burst` | int   | 2000           | Maximum burst size for throttling Kubernetes API requests. Allows temporarily exceeding the QPS limit when there are sudden spikes in request volume                 |
| `k8s-skr-client-qps`   | int   | 50           | Maximum queries per second (QPS) limit for the SKR Kubernetes client. Controls how many requests can be made to the Kubernetes API server per second in the steady state |
| `k8s-skr-client-burst` | int   | 100           | Maximum burst size for throttling SKR Kubernetes API requests. Allows temporarily exceeding the QPS limit when there are sudden spikes in request volume                 |

## Certificates Configuration

| Flag                                | Type     | Default Value          | Description                                                                                                          |
|-------------------------------------|----------|------------------------|----------------------------------------------------------------------------------------------------------------------|
| `cert-management`                   | string   | cert-manager.io/v1     | Certificate management system to use. Accepted values: `cert-manager.io/v1`, `cert.gardener.cloud/v1alpha1`          |
| `self-signed-cert-duration`         | duration | 90*24h                 | Duration of self-signed certificate. Minimum: 1h                                                                     |
| `self-signed-cert-renew-before`     | duration | 60*24h                 | Duration before the currently issued self-signed certificate's expiry when cert-manager should renew the certificate |
| `self-signed-cert-renew-buffer`     | duration | 24h                    | Duration to wait before confirming self-signed certificate are not renewed                                           |
| `self-signed-cert-key-size`         | int      | 4096                   | Key size for the self-signed certificate                                                                             |
| `self-signed-cert-issuer-name`      | string   | klm-watcher-selfsigned | Issuer name for the self-signed certificate                                                                          |
| `self-signed-cert-naming-template`  | string   | %s-webhook-tls         | Naming template for the self-signed certificate. Should contain one `%s` placeholder for the Kyma name               |
| `self-signed-cert-issuer-namespace` | string   | istio-system           | Namespace of the Issuer for self-signed certificates                                                                 |

## Istio Gateway Configuration

| Flag                                               | Type     | Default Value | Description                                                                                                  |
|----------------------------------------------------|----------|---------------|--------------------------------------------------------------------------------------------------------------|
| `istio-gateway-cert-switch-before-expiration-time` | duration | 24h           | Duration before the expiration of the current CA certificate when the Gateway certificate should be switched |
| `istio-namespace`                                  | string   | istio-system  | Namespace for Istio resources in a cluster                                                                   |
| `istio-gateway-name`                               | string   | klm-watcher   | Name of the Istio Gateway resource in a cluster                                                              |
| `istio-gateway-namespace`                          | string   | kcp-system    | Namespace for the Istio Gateway resource in a cluster                                                        |
| `legacy-strategy-for-istio-gateway-secret`         | bool     | false         | Use the legacy strategy (with downtime) for the Istio Gateway Secret                                         |

## Metrics and Health Configuration

| Flag                        | Type     | Default Value | Description                                                                      |
|-----------------------------|----------|---------------|----------------------------------------------------------------------------------|
| `metrics-bind-address`      | string   | :8080         | Address and port for binding of metrics endpoint                                 |
| `metrics-cleanup-interval`  | int      | 15            | Interval (in minutes) at which the cleanup of non-existing Kyma CRs metrics runs |
| `health-probe-bind-address` | string   | :8081         | Address and port for binding of health probe endpoint                            |
| `pprof-bind-address`        | string   | :8084         | Address and port for binding of pprof profiling endpoint                         |
| `pprof`                     | bool     | false         | Enable a pprof server                                                            |
| `pprof-server-timeout`      | duration | 90s           | Duration of timeout of read/write for the pprof server                           |

## Runtime Watcher Configuration

| Flag                                 | Type   | Default Value                           | Description                                                                                                                                                                                 |
|--------------------------------------|--------|-----------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `skr-watcher-image-name`             | string | runtime-watcher                         | Image name to be used for the SKR Watcher image                                                                                                                                             |
| `skr-watcher-image-tag`              | string | ""                                      | Image tag to be used for the SKR Watcher image                                                                                                                                              |
| `skr-watcher-image-registry`         | string | europe-docker.pkg.dev/kyma-project/prod | Image registry to be used for the SKR Watcher image                                                                                                                                         |
| `skr-webhook-memory-limits`          | string | 200Mi                                   | Resource limit for memory allocation to the SKR webhook                                                                                                                                     |
| `skr-webhook-cpu-limits`             | string | 0.1                                     | Resource limit for CPU allocation to the SKR webhook                                                                                                                                        |
| `skr-watcher-path`                   | string | ./skr-webhook                           | Path to the static SKR Watcher resources                                                                                                                                                    |
| `kyma-skr-listener-bind-address`     | string | :8082                                   | Address and port for binding the SKR event listener for Kyma resources                                                                                                                      |
| `manifest-skr-listener-bind-address` | string | :8083                                   | Address and port for binding the SKR event listener for Manifest resources                                                                                                                  |
| `additional-dns-names`               | string | ""                                      | Additional DNS Names which are added to SKR certificates as SANs. Input should be given as comma-separated list, for example "--additional-dns-names=localhost,127.0.0.1,host.k3d.internal" |
| `listener-port-overwrite`            | string | ""                                      | Port that is mapped to HTTP port of the local k3d cluster using --port 9443:443@loadbalancer when creating the KCP cluster                                                                  |

## Leader Election Values

| Flag                             | Type     | Default Value | Description                                                                                                                     |
|----------------------------------|----------|---------------|---------------------------------------------------------------------------------------------------------------------------------|
| `leader-elect`                   | bool     | false         | Enable leader election for controller manager. Enabling it ensures there is only one active controller manager                  |
| `leader-election-lease-duration` | duration | 180s          | Duration configured for the 'LeaseDuration' option of the controller-runtime library used to run the controller manager process |
| `leader-election-renew-deadline` | duration | 120s          | Duration configured for the 'RenewDeadline' option of the controller-runtime library used to run the controller manager process |
| `leader-election-retry-period`   | duration | 3s            | Duration configured for the 'RetryPeriod' option of the controller-runtime library used to run the controller manager process   |


## Purge Configuration

| Flag                         | Type     | Default Value | Description                                                                                                           |
|------------------------------|----------|---------------|-----------------------------------------------------------------------------------------------------------------------|
| `enable-purge-finalizer`     | bool     | false         | Enable Purge controller. Please make sure to provide a value for `skip-finalizer-purging-for` if this flag is enabled |
| `purge-finalizer-timeout`    | duration | 5m            | Duration after a Kyma's deletion timestamp when the remaining resources should be purged in the SKR                   |
| `skip-finalizer-purging-for` | string   | ""            | CRDs to be excluded from finalizer removal. Example: 'ingressroutetcps.traefik.containo.us,*.helm.cattle.io'          |

## Miscellaneous Configuration

| Flag                          | Type     | Default Value                                                        | Description                                                                                                                                                                  |
|-------------------------------|----------|----------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `min-maintenance-window-size` | duration | 20m                                                                  | Minimum duration of maintenance window required for reconciling modules with downtime                                                                                        |
| `drop-crd-stored-version-map` | string   | Manifest:v1beta1,Watcher:v1beta1,ModuleTemplate:v1beta1,Kyma:v1beta1 | API versions to be dropped from the storage version. The input format must be a comma-separated list of API versions, where each API version is in the `kind:version` format |
| `is-kyma-managed`             | bool     | false                                                                | Use managed Kyma mode                                                                                                                                                        |
| `sync-namespace`              | string   | kyma-system                                                          | Namespace for syncing remote Kyma and module catalog                                                                                                                         |
| `enable-webhooks`             | bool     | false                                                                | Enable Validation/Conversion Webhooks                                                                                                                                        |
| `log-level`                   | int      | 0 (Warn level)                                                       | Log level. Enter negative or positive values to increase verbosity. 0 has the lowest verbosity.                                                                              |
| `oci-registry-cred-secret`    | string   | ""                                                                   | Allows to configure name of the Secret containing the OCI registry credential.                                                                                               |
