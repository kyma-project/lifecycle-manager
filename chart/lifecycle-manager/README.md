# lifecycle-manager Helm Chart

This is the in-repo Helm chart for deploying the [Kyma Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) (KLM) controller. It is the single source of truth for the KLM operator objects, used for both local end-to-end testing and, through a production wrapper chart, for deployment to Kyma Control Plane (KCP) clusters.

## Scope

This chart is the **core chart**: it owns the operator objects that KLM needs to run. Site reliability engineering (SRE) and landscape concerns live in a separate production wrapper chart in the [management-plane-charts](https://github.tools.sap/kyma/management-plane-charts) repository, which consumes this chart as a subchart.

The following objects are owned by this core chart:

| Template | Resources |
|---|---|
| `deployment.yaml` | Controller manager `Deployment` |
| `serviceaccount.yaml` | `ServiceAccount`, `Role`s, `ClusterRole`, and bindings |
| `crds.yaml` | `CustomResourceDefinition`s |
| `certificate.yaml` | `Certificate` and `Issuer` objects for the watcher CA and webhook |
| `service.yaml` | `Service`s for the listener, events, metrics, and webhook |
| `watcher.yaml` | `Watcher` custom resource |
| `gateway.yaml` | Istio `Gateway` |
| `authorizationpolicy.yaml` | Istio `AuthorizationPolicy` |

The following SRE and landscape objects are **not** part of this core chart. They remain owned by the production wrapper chart:

| Object | Wrapper template |
|---|---|
| Plutono dashboard `ConfigMap`s | `dashboards.yaml` |
| `lifecycle-manager-istio` `Service` (port 15020) | `service.yaml` (istio-proxy admin block) |
| `NetworkPolicy`s | `network-policies.yaml` |
| `PrometheusRule` and `VMRule` | `prometheus-rules.yaml` |
| `VMServiceScrape` and `ServiceMonitor` | `vmscrapes.yaml` |
| ArgoCD `Application` | `argoapp.yaml` |
| Promotion workflow | `promotion/` |

## Certificate backend

The chart supports two certificate backends and selects between them based on cluster capabilities. When the `cert-manager.io/v1` API is available, the chart renders cert-manager objects. Otherwise, it renders Gardener certificate objects. This branch is handled in the templates through `.Capabilities.APIVersions.Has "cert-manager.io/v1"`, so no value toggle is required.

## Generated content

The CRDs and RBAC rules under `files/` are generated from the upstream KLM sources, so they stay in sync with `make manifests` output. See the `chart-sync` make target for the generation mechanics.

## Parity with production

The chart's production render must not change during the migration. The `scripts/chart-parity` gate proves that this chart renders semantically identically to the production chart, for both certificate backends. Run it after any template or values change.
