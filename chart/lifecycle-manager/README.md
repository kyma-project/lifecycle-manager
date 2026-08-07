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

The CRDs and generated RBAC rules under `files/` are produced from the KLM sources by `make chart-sync`, so they stay in sync without a manual copy step. They are **not committed** to the repository (see `.gitignore`); you must generate them before you render or deploy the chart. With them absent, `helm` renders successfully but silently emits zero CRDs and empty RBAC rules, so the e2e deploy path runs `chart-sync` before every `helm` invocation.

- `files/imports/crds.yaml` comes from `kustomize build config/control-plane`, filtered to the five shipped CRDs. The control-plane build is required rather than a raw `config/crd/bases` concatenation, because the chart's CRDs carry metadata layered by kustomize: the `cert-manager.io/inject-ca-from` annotation and the `app.kubernetes.io/*` labels.
- `files/rbac-rules/klm-manager-rules.yaml`, `klm-manager-rules-crd.yaml`, and `klm-leader-election-rules.yaml` hold the `.rules` arrays extracted from the hand-maintained `config/rbac/*.yaml` source files. These are generated and not committed.
- `files/rbac-rules/klm-certmanager-rules.yaml` and `klm-gardener-cm-rules.yaml` are static and committed. They have no `config/rbac` counterpart, so `make chart-sync` leaves them untouched.
- `files/cert-keys/*.yaml` are static and committed.

## Parity with production

During the migration, a local-only harness under `scripts/chart-parity/` verifies that this chart renders semantically identically to the production chart, for both certificate backends. The harness is development scaffolding and is not committed; see its README for details.
