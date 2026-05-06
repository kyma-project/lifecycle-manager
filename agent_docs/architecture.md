# Architecture

## Component map

```
kyma-operator-manager/ (monorepo)
├── lifecycle-manager/          ← this operator (runs on KCP)
│   ├── api/                    ← separate Go module; CRD types only
│   ├── internal/               ← controllers, services, repositories
│   ├── pkg/                    ← reusable packages (queue, status, templatelookup, watcher, …)
│   ├── cmd/                    ← main.go + dependency-injection composition
│   ├── config/                 ← kustomize bases: crd/, rbac/, webhook/, manager/, overlays/
│   ├── tests/integration/      ← per-controller envtest suites
│   ├── maintenancewindows/     ← separate Go module
│   └── skr-webhook/            ← separate Go module
│
├── template-operator/          ← reference module operator (used in tests)
├── modulectl/                  ← CLI for scaffolding/publishing modules
└── runtime-watcher/listener/   ← library consumed by lifecycle-manager
```

## KCP / SKR split

lifecycle-manager operates across two cluster roles:

| Term | Meaning |
|---|---|
| **KCP** | Kyma Control Plane — the cluster where lifecycle-manager runs |
| **SKR** | Satellite Kyma Runtime — a customer-managed cluster |

Each SKR is represented on KCP by a `Kyma` CR. The `Kyma` spec is the desired state; it is
populated from the SKR-side `Kyma` copy on every reconcile (`replaceSpecFromRemote`). Lifecycle-
manager installs/removes modules on the SKR by creating `Manifest` CRs on KCP; the manifest
controller then applies the actual workloads to the SKR.

Connectivity to SKRs goes through `SkrContextFactory` (`internal/remote/`). It caches REST
clients per Kyma name and invalidates them on auth errors.

## Controllers

| Controller | Package | Owns | Watches |
|---|---|---|---|
| `kyma` | `internal/controller/kyma` | `Kyma` | `Kyma`, `ModuleTemplate`, `ModuleReleaseMeta` |
| `manifest` | `internal/controller/manifest` | `Manifest` | `Manifest` |
| `watcher` | `internal/controller/watcher` | `Watcher` | `Watcher` |
| `mandatorymodule/installation` | `internal/controller/mandatorymodule` | `Manifest` | `Kyma` |
| `mandatorymodule/deletion` | `internal/controller/mandatorymodule` | `Manifest` | `Kyma` |
| `purge` | `internal/controller/purge` | `Kyma` | `Kyma` |
| `istiogatewaysecret` | `internal/controller/istiogatewaysecret` | Secret | Secret |

## Service layer

Business logic lives in `internal/service/`, not in controllers. Controllers call services;
services do not call other services directly.

Key services and their jobs:

| Service | Location | Job |
|---|---|---|
| `SkrContextFactory` | `internal/remote/` | Provide authenticated SKR clients |
| `RemoteCatalog` | `internal/remote/` | Sync `ModuleTemplate` CRs to SKR |
| `SKRWebhookManager` | `internal/service/watcher/` | Install/remove runtime-watcher webhook on SKR |
| `SkrSyncService` | `internal/service/skrsync/` | Sync CRDs and image-pull secrets to SKR |
| `DeletionService` | `internal/controller/kyma/deletion/` | Orchestrate Kyma deletion steps |
| `RestrictedModules` | `internal/service/restrictedmodule/` | Default restricted module entries |

## Watcher integration

The `runtime-watcher` component runs on each SKR and pushes change events back to KCP via
a webhook. lifecycle-manager installs this webhook through `SKRWebhookManager`. The `Watcher`
CRD (`api/v1beta2`) configures which resources on the SKR are watched and where events are
forwarded. The watcher listener library (`github.com/kyma-project/runtime-watcher/listener`) is
consumed directly in `internal/controller/watcher/`.

## Dependency injection

`cmd/composition/` holds pure-function composers that wire up each controller from its
dependencies. No `init()` side effects. The composition functions (`ComposeKymaDeletionService`,
`ComposeSkrWebhookManager`, etc.) are also called from integration test suites to get
production-equivalent wiring under envtest.
