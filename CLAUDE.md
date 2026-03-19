# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

Kyma Lifecycle Manager (KLM) is a Kubernetes operator built with controller-runtime that manages the lifecycle of Kyma modules running on SAP BTP Kyma runtimes (SKRs).
It handles module management, like provisioning, module resources sync and deletion. The operator runs on a central control plane (KCP) and manages remote SKR (SAP Kyma Runtime) clusters. The main controllers are Kyma controller and Manifest controller, where Kyma contains the general status of the module installation and Kyma cluster, and the Manifest contains the module details.
Modules are OCM (Open Component Model) bundled resources, that are versioned (Component Version) and released in a unmodifiabable way. Users enable and disable modules generally by modifying the Kyma CR on the SKR cluster, where our skr-webhook (watcher) component listens for changes and sends reconciliation requests to KLM on KCP.

## Build and Development Commands

The project uses `GOFIPS140=v1.0.0 go` for all Go commands (FIPS-enabled builds).

```bash
# Build
make build                    # Generate code, format, vet, build binary to bin/manager

# Run all tests (unit + integration with envtest)
make test

# Run only unit tests (main module, excludes tests/ directory)
make unittest-klm

# Run only api module unit tests
make unittest-api             # or: cd api && make test

# Run only maintenancewindows module unit tests
make unittest-maintenancewindows  # or: cd maintenancewindows && make test

# Run a single test or package
GOFIPS140=v1.0.0 go test ./internal/controller/kyma/... -run TestSpecificName -v

# Integration tests (requires envtest assets)
KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.32.0 -p path)" GOFIPS140=v1.0.0 go test ./tests/integration/...

# Generate CRDs, RBAC, webhook configs
make manifests

# Generate DeepCopy methods
make generate

# Lint
make lint                     # Runs golangci-lint on all three modules (root, api, maintenancewindows)
make fmt                      # go fmt
make vet                      # go vet

# Run controller locally against a cluster
make run
```

## Multi-Module Structure

The repository contains three Go modules with local `replace` directives in the root `go.mod`:

- **Root module** (`github.com/kyma-project/lifecycle-manager`) — the operator itself
- **`api/`** (`github.com/kyma-project/lifecycle-manager/api`) — CRD types, separate module so consumers can import types without the full operator dependency tree
- **`maintenancewindows/`** (`github.com/kyma-project/lifecycle-manager/maintenancewindows`) — maintenance window resolution logic

Each sub-module has its own `go.mod`, `go.sum`, and `Makefile`. Linting runs against all three. Tool versions are centralized in `versions.yaml`.

## Architecture

### Dual-Cluster Model

KLM runs on a **control plane (KCP) cluster** and manages remote **SKR clusters**. This is a fundamental architectural concept:
- KCP holds `Kyma`, `ModuleTemplate`, `ModuleReleaseMeta`, `Manifest`, and `Watcher` CRs
- SKR clusters are managed remotely via kubeconfig secrets stored in KCP
- `internal/remote/` handles SKR client creation, caching, and CRD syncing

### CRD Types (`api/v1beta2/`)

- **Kyma** — represents a managed Kyma instance (one per SKR cluster). Spec defines desired modules/channel
- **Manifest** — represents a single module installation. Created by the Kyma controller, reconciled by the Manifest controller to install Helm charts on SKR
- **ModuleTemplate** — defines a module version with its component descriptor and default CR
- **ModuleReleaseMeta** — maps module versions to channels
- **Watcher** — configures runtime watcher webhooks on SKR clusters

Shared constants, labels, annotations, and state types live in `api/shared/`.

### Controllers (`internal/controller/`)

- **kyma** — main reconciler; resolves modules from templates, creates/updates Manifest CRs, syncs CRDs and catalogs to SKR, manages finalizers
- **kyma/deletion** — handles Kyma CR deletion lifecycle
- **manifest** — installs module workloads on SKR clusters using the declarative library; delegates to `internal/declarative/v2/`
- **mandatorymodule** — two controllers: `InstallationReconciler` (ensures mandatory modules) and `DeletionReconciler` (cleans up deleted mandatory modules)
- **purge** — force-deletes all module resources on SKR when Kyma is stuck in deletion
- **watcher** — manages runtime watcher webhook configurations and Istio virtual services
- **istiogatewaysecret** — rotates TLS certificates for the Istio gateway

### Layered Architecture Pattern

Well-structured services follow: **Controller → Service → Repository**, wired through the **Composition Root** (`cmd/composition/`):

- **`cmd/composition/`** — factory functions (`Compose*`) that wire repositories into services. Organized by `provider/`, `repository/`, and `service/` subdirectories
- **`internal/service/`** — business logic layer (kyma deletion/lookup, mandatory module installation/deletion, SKR webhook management, SKR client management, etc.)
- **`internal/repository/`** — data access layer for KCP resources (kyma, manifest, secret, moduletemplate, modulereleasemeta, istiogateway, watcher, OCM descriptors) and SKR resources (`skr/`)
- **`internal/result/`** — typed `Result` struct for use-case-driven control flow

### Key Internal Packages

- **`internal/declarative/v2/`** — generic reconciliation engine for Manifest CRs (Helm chart rendering, resource application, state checking)
- **`internal/remote/`** — SKR cluster connectivity: `SkrContext`, `SkrContextProvider`, client caching, remote catalog syncing, CRD upgrade logic
- **`internal/manifest/`** — Helm/OCI spec resolution, image rewriting, keychain providers, state checks for deployments/statefulsets
- **`internal/pkg/flags/`** — CLI flag definitions for the operator binary
- **`internal/pkg/metrics/`** — Prometheus metrics for all controllers
- **`pkg/`** — public utilities: `templatelookup` (module resolution), `queue` (requeue intervals), `watcher` (SKR webhook manifest manager), `testutils` (e2e test helpers)

### Event/Result Pattern

Controllers use `internal/event/` for Kubernetes event recording and `internal/result/` for typed use-case results, composed via `internal/result/event/` which converts results into Kubernetes events.

## Testing

- **Unit tests**: Co-located with source files (`*_test.go`). Use `testify` for assertions. Run with `make unittest-klm`
- **Integration tests** (`tests/integration/`): Use controller-runtime's `envtest` to run controllers against a real API server. Use Ginkgo/Gomega. Organized by controller under `tests/integration/controller/`
- **E2E tests** (`tests/e2e/`): Run against real KCP+SKR clusters (k3d). Require `KCP_KUBECONFIG` and `SKR_KUBECONFIG` env vars. Use Ginkgo/Gomega

## Code Conventions

- **Import aliases**: Strictly enforced via `importas` linter — see `.golangci.yaml` for the full alias map. Key ones: `apimetav1`, `apicorev1`, `apierrors`, `ctrl`, `ctrlruntime`, `machineryruntime`
- **Import ordering**: standard → third-party → project (`github.com/kyma-project/lifecycle-manager`) → blank → dot (enforced by `gci`)
- **Line length**: 120 characters (revive `line-length-limit`)
- **Max function length**: 80 lines (`funlen`)
- **Max cyclomatic complexity**: 20 (`cyclop`)
- **All linters enabled by default** with specific exclusions — check `.golangci.yaml` before adding `//nolint` directives
- **`//nolint` requires explanation**: e.g., `//nolint:funlen // composition root wiring`

## Documentation Guidelines

When adding, updating, or removing any documentation inside the `docs/` folder, you must always follow the guidelines in [docs/claude-docs.md](docs/CLAUDE.md).
