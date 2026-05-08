# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Module & language

- Main module: `github.com/kyma-project/lifecycle-manager` (Go 1.26.1, per `versions.yaml`)
- The API types live in a **separate Go module**: `github.com/kyma-project/lifecycle-manager/api` — run `go` commands targeting API code from within `api/`, not from the repo root.
- Additional sub-modules: `maintenancewindows/`, `skr-webhook/`
- Key dependencies: `controller-runtime v0.23.3`, `k8s.io/* v0.35.4`, `cert-manager v1.20.2`, `istio v1.29.2`, `ocm.software/ocm v0.40.0`, `runtime-watcher/listener v1.4.0`
- Tool versions are pinned in `versions.yaml`; `make <tool>` downloads and caches them to `bin/`.

## Common make targets

Run from `lifecycle-manager/`. All `go` commands use `GOFIPS140=v1.0.0 go` (FIPS-enabled builds) — the Makefile sets this automatically, but use it explicitly when running `go` commands directly.

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
GOFIPS140=v1.0.0 go test -run TestFoo ./internal/...
```

Single controller's integration tests (requires envtest assets):
```sh
KUBEBUILDER_ASSETS=$(./bin/setup-envtest use 1.32.0 -p path) \
  GOFIPS140=v1.0.0 go test ./tests/integration/controller/kyma/... -v -ginkgo.focus "some spec description"
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

## Code conventions

### Import aliases

The `importas` linter enforces strict aliases — violations fail CI. Key ones you will use in almost every file:

| Package | Alias |
|---|---|
| `k8s.io/apimachinery/pkg/apis/meta/v1` | `apimetav1` |
| `k8s.io/apimachinery/pkg/api/errors` | `apierrors` |
| `k8s.io/apimachinery/pkg/runtime` | `machineryruntime` |
| `k8s.io/apimachinery/pkg/labels` | `k8slabels` |
| `k8s.io/api/core/v1` | `apicorev1` |
| `k8s.io/api/apps/v1` | `apiappsv1` |
| `k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1` | `apiextensionsv1` |
| `sigs.k8s.io/controller-runtime` | `ctrl` |
| `sigs.k8s.io/controller-runtime/pkg/controller` | `ctrlruntime` |
| `sigs.k8s.io/controller-runtime/pkg/log` | `logf` |

Full alias list is in `.golangci.yaml` under `linters-settings.importas.alias`.

### Import ordering

Enforced by `gci`: **standard → third-party → project** (`github.com/kyma-project/lifecycle-manager`) **→ blank → dot**.

### Lint limits

- Line length: **120 characters** (revive `line-length-limit`)
- Function length: **80 lines** (`funlen`) — use `//nolint:funlen // <reason>` only for composition root wiring
- Cyclomatic complexity: **20** (`cyclop`)
- All linters enabled by default; check `.golangci.yaml` before adding `//nolint`. Every `//nolint` directive **must** include an explanation comment.

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
| Documentation writing style and templates | [`docs/CLAUDE.md`](docs/CLAUDE.md) |

## Security guardrails

These constraints exist for specific CVE mitigations or compliance requirements — do not remove or weaken them without understanding what they protect against.

### FIPS compliance
- **Never remove `GOFIPS140=v1.0.0`** from `Dockerfile` or `Makefile`. Mandatory for SAP/Kyma production builds. The Go FIPS module restricts crypto to FIPS-140-approved algorithms.
- FIPS mode is monitored at runtime via the `lifecycle_mgr_fips_mode` Prometheus metric (`internal/pkg/metrics/fipsMode.go`). A value of `0` means FIPS is off — that is an incident.
- Do not add Go dependencies that use non-FIPS-approved crypto (custom cipher suites, `golang.org/x/crypto` elliptic curves that bypass the stdlib FIPS module).

### TLS enforcement
- **`config/watcher/gateway.yaml`** enforces TLS 1.3 exclusively (`minProtocolVersion: TLSV1_3`, `maxProtocolVersion: TLSV1_3`) with `mode: MUTUAL`. Do not downgrade to TLS 1.2.
- **`forwardClientCertDetails: SANITIZE_SET`** on the Istio Gateway prevents client cert header spoofing — keep it.
- Certificates use 4096-bit RSA with `rotationPolicy: Always` (key re-generated on every renewal). Do not reduce key size or remove the rotation policy. See `config/certmanager/certificate_watcher.yaml`.

### Container security context
Every container (including sidecars and init containers) must include:
```yaml
securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  capabilities:
    drop: ["ALL"]
  seccompProfile:
    type: RuntimeDefault
```
Reference: `skr-webhook/resources.yaml` and `config/manager/manager.yaml`.

### Container base image
`Dockerfile` pins `gcr.io/distroless/static:nonroot` to a **sha256 digest**. Never switch to a tag (`:latest`, `:nonroot`) — only the digest form is acceptable. When updating the base image, update the digest and document the CVE or reason.

### NetworkPolicies
`skr-webhook/resources.yaml` contains four strict NetworkPolicies. Do not relax egress to `0.0.0.0/0` or remove namespace/pod selectors. All rules are intentional:
- Ingress: Gardener VPN source only + Prometheus scrape port
- Egress: Kubernetes API server (443) + DNS only

### RBAC
`config/rbac/manager_role.yaml` uses explicit resource and verb lists — no wildcards. When adding a permission: use the minimum verb set, add a separate `rules` entry per API group. Never use `resources: ["*"]` or `verbs: ["*"]`.

### Secret handling
TLS keys and sensitive credentials must be mounted as Kubernetes Secret volumes — never passed as environment variables. See `config/certmanager/certificate_watcher.yaml` for the cert-manager pattern.

### CVE triage
Three scanners run against this repo (`sec-scanners-config.yaml`): **Checkmarx One** (SAST), **BDBA** (container CVE scan), **Mend** (Go module SCA). When triaging a CVE finding, see [`.claude/cve-triage/context.md`](.claude/cve-triage/context.md).

## Model usage

Follow the Kyma team's Claude Code workflow:

- **Planning complex tasks** — switch to Opus: `/model claude-opus-4-7`
- **Implementation** — use the default Sonnet: `/model claude-sonnet-4-6`

Use Opus when you need to understand an unfamiliar subsystem, design a non-trivial change, or reason about cross-cutting impacts. Switch back to Sonnet once the approach is clear and you are writing code.
