# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Module & language

- Main module: `github.com/kyma-project/lifecycle-manager` (Go 1.26.1, per `versions.yaml`)
- The API types live in a **separate Go module**: `github.com/kyma-project/lifecycle-manager/api` — run `go` commands targeting API code from within `api/`, not from the repo root.
- Additional sub-modules: `maintenancewindows/`, `skr-webhook/`
- Key dependencies: `controller-runtime v0.23.3`, `k8s.io/* v0.35.4`, `cert-manager v1.20.2`, `istio v1.29.2`, `ocm.software/ocm v0.40.0`, `runtime-watcher/listener v1.4.0`
- Tool versions are pinned in `versions.yaml`; `make <tool>` downloads and caches them to `bin/`.

## Common make targets

Run from `lifecycle-manager/`:

| Target | What it does |
|---|---|
| `make generate` | Regenerate `zz_generated.deepcopy.go` via controller-gen |
| `make manifests` | Regenerate CRD YAML (`config/crd/bases/`) and RBAC (`config/rbac/`) via controller-gen |
| `make test` | Unit tests + envtest integration tests (also runs generate/manifests/fmt/vet) |
| `make unittest-klm` | Unit tests only for the main module (no envtest) |
| `make unittest-api` | Unit tests for the `api/` sub-module |
| `make build` | Compile `bin/manager` |
| `make lint` | golangci-lint across main module, `api/`, and `maintenancewindows/` |
| `make fmt` | `go fmt ./...` |

**After any change to a type in `api/`**: run both `make generate` and `make manifests`.
A CI workflow (`check-generated-code.yml`) blocks PRs where generated files are out of sync.

### Running a single test

Unit test:
```sh
go test -run TestFoo ./internal/...
```

Single controller's integration tests (requires envtest assets):
```sh
KUBEBUILDER_ASSETS=$(./bin/setup-envtest use 1.32.0 -p path) \
  go test ./tests/integration/controller/kyma/... -v -ginkgo.focus "some spec description"
```

Replace `kyma` with `manifest`, `watcher`, `modulereleasemeta`, or `moduletemplate`. Run `make envtest` once after checkout to populate `bin/setup-envtest`.

## Architecture overview

See [`agent_docs/architecture.md`](agent_docs/architecture.md) for the full component map.

- lifecycle-manager runs on **KCP** (Kyma Control Plane) and manages a fleet of **SKR** clusters (Satellite Kyma Runtimes).
- The `Kyma` CR on KCP is the source of truth for *which modules to install*; its `.spec` is **overwritten** from the remote SKR copy on every reconcile (`remote.ReplaceSpec`). Never depend on the KCP-side spec persisting across reconcile calls.
- Controllers: `kyma`, `manifest`, `watcher`, `mandatorymodule` (install + delete), `purge`, `istiogatewaysecret`.

### Module installation flow

`Kyma.spec.modules` → `ModuleReleaseMeta` (channel → version mapping) → `ModuleTemplate` (OCI descriptor) → `Manifest` CR (created on KCP with OwnerReference to `Kyma`) → manifest controller applies workloads to SKR.

If no `ModuleReleaseMeta` exists for a module, the controller falls back to listing all `ModuleTemplate` CRs and filtering by `spec.channel`. Missing channel entries put the Kyma CR into `Error` state.

### Mandatory modules

Fetched via the `operator.kyma-project.io/mandatory-module` label on `ModuleTemplate`. No channel concept; highest version wins when multiple exist. Mandatory module status does not appear in `Kyma.status`. Deletion is handled by a separate controller that adds a finalizer to the `ModuleTemplate` and waits for all associated `Manifest` CRs to be gone before releasing it.

### Purge controller

Forcefully removes all module resources from a remote cluster when a `Kyma` CR has been stuck in deletion longer than the grace period (default: 5 minutes). Removes finalizers from all remote CRs so garbage collection can proceed.

## Architectural guardrails

1. **Reconcilers must not mutate `.spec`** of the object they own. The only exceptions are `EnsureLabelsAndFinalizers` (labels/finalizers) and `replaceSpecFromRemote` (Kyma spec). Status is written freely.

2. **Communicate state via conditions, not free-form strings.** Use `kyma.UpdateCondition(type, status)` with the shared condition types in `api/v1beta2` (`ConditionTypeModules`, `ConditionTypeModuleCatalog`, `ConditionTypeSKRWebhook`, etc.).

3. **controller-gen markers are the source of truth for CRD schema.** Never hand-edit files in `config/crd/bases/`. See [`agent_docs/crd-conventions.md`](agent_docs/crd-conventions.md).

4. **All services are injected via interfaces.** The `Reconciler` struct holds interface fields (`SkrContextFactory`, `SKRWebhookManager`, `DeletionService`, etc.). Add new dependencies as interfaces; never call concrete types directly from the reconcile loop.

5. **Requeueing uses `queue.DetermineRequeueInterval`**, not hardcoded durations. Short explicit intervals (e.g. `1 * time.Second`) are only used during deletion transitions.

6. **Error wrapping**: always use `fmt.Errorf("context: %w", err)`. For deletion use cases, return `result.Result{UseCase: usecase.X, Err: err}` and let the caller decide on requeue/metrics.

## Where to look for more context

| Topic | File |
|---|---|
| Component map, KCP/SKR split, DI wiring | [`agent_docs/architecture.md`](agent_docs/architecture.md) |
| Reconciler patterns, state machine, SKR context | [`agent_docs/reconcilers.md`](agent_docs/reconcilers.md) |
| CRD naming, versioning, markers | [`agent_docs/crd-conventions.md`](agent_docs/crd-conventions.md) |
| Running tests, envtest setup, Ginkgo conventions | [`agent_docs/testing.md`](agent_docs/testing.md) |
| Code generation, when to run it, troubleshooting drift | [`agent_docs/codegen.md`](agent_docs/codegen.md) |
| Controller responsibilities in depth | [`docs/contributor/02-controllers.md`](docs/contributor/02-controllers.md) |
| KCP↔SKR synchronization protocol | [`docs/contributor/08-kcp-skr-synchronization.md`](docs/contributor/08-kcp-skr-synchronization.md) |
