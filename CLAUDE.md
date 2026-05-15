# CLAUDE.md

Kyma Lifecycle Manager (KLM) is a Kubernetes operator built with kubebuilder and controller-runtime in Go.
It manages the lifecycle of Kyma modules running on *SAP BTP, Kyma Runtimes* (SKRs) including their installation, update and uninstallation.
KLM runs on a central **control plane (KCP)** Kubernetes cluster and manages remote **SKR** Kubernetes clusters.
Modules are [OCM](https://ocm.software/) packaged components, typically containing a manifest of Kubernetes resources including a module operator.

To build KLM, run `make build`.

## KLM works with the following Custom Resources (CRs)

- **Kyma** - defines what modules shall be installed on the SKR. The user chooses a channel, not a specific version of the module. Lives on KCP and SKR.
- **ModuleTemplate** - defines the metadata of a module version. Lives on KCP and SKR.
- **ModuleReleaseMeta** - assigns module versions to channels or defines the module as mandatory and which version to install. Lives on KCP and SKR.
- **Manifest** - defines a single installation of a module on a specific SKR. Lives on KCP only.
- **Watcher** - configures a webhook to be installed on the SKR. The webhook notifies KLM of changes to resources on the SKR.

These are defined in a separate Go module in `api/v1beta2`.

## KLM uses the following controllers

- **Kyma controller** - syncs essential resources to the SKR. Creates, updates and deletes Manifest CRs. Tracks the overall status of module installations and the SKR.
- **Manifest controller** - installs, updates and uninstalls module resources on the SKR. Tracks the status of the module installation.
- **Mandatory Module Installation controller** - creates and updates Manifest CRs for mandatory modules.
- **Mandatory Module Deletion controller** - deletes Manifest CRs for mandatory modules.
- **Purge controller** - purges Custom Resource Definitions from SKRs after a certain timeout when these block the deprovisioning of the SKR.
- **Watcher controller** - installs the watcher webhook to the SKR and configures ingress on KCP.
- **Istio Gateway Secret controller** - manages the certificate rotation in the Public Key Infrastructure (PKI) inbound Watcher traffic.

These are defined in `internal/controller`.

## KLM follows the following key architectural decisions

- **ADR 001** - prefer Consumer-Defined Interfaces.
- **ADR 002** - inject Dependencies via Constructor in Composition Functions.
  - the composition root is resolved in `cmd/main.go`
- **ADR 003** - use the most specific client interface from controller-runtime and use it at the Repository layer only.
- **ADR 004** - adopt a Layered Architecture consisting of Controller -> Service -> Repository layers. No layer may reference or depend on a higher layer.
  - the layers are represented in `internal/controller`, `internal/service`, `internal/repository`
- **ADR 005** - suffix types with Controller, Service, Repository; don't suffix with Interface or Impl; let the package provide the context.

These are defined in `docs/contributor/adr`.
Only load the entire ADR if considered **relevant** for the task.
Not the entire project follows the ADRs yet, but all **new** code **MUST** follow them.

## KLM uses a unit, integration and e2e test pyramid

- **Unit tests** - are co-located with the source files as `*_test.go` and defined in a `*_test` package.
  - to run all unit tests for KLM: `gmake unittest-klm`
  - to run a specific test: `go test <path> -run <test name> -v`
- **Integration tests** - are located in `tests/integration`
  - to run all integration tests: `gmake test`
  - to run a specific suite: `KUBEBUILDER_ASSETS="$(./bin/setup-envtest use <envtest_k8s from versions.yaml> -p path)" go test <path> -v`
- **E2E tests** - are located in `tests/e2e`
  - to run a specific test `gmake -f ./tests/e2e/Makefile <test name>`
    - note that only those tests forwarding to a dedicated makefile are supported like this yet

## Code Conventions

Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/) as a baseline.

Project-specific rules enforced by `golangci-lint` (see `.golangci.yaml`):
- **Import aliases**: strictly enforced — key ones: `apimetav1`, `apicorev1`, `apierrors`, `machineryruntime`, `ctrl`, `ctrlruntime`
- **Import ordering** (gci): standard → third-party → project (`github.com/kyma-project/lifecycle-manager`) → blank → dot
- **Line length**: 120 chars | **Function length**: 80 lines | **Cyclomatic complexity**: 20
- **All linters enabled by default** — check `.golangci.yaml` before adding `//nolint`
- **`//nolint` requires explanation**: e.g., `//nolint:funlen // composition root wiring`

## Commits and Pull Requests

- PRs are usually created from a fork branch against main, exceptions are working on upstream feature branches for collaboration on bigger features.
- PRs will be merged with squash merge, so the PR title and description will form the commit message.
- Always keep [conventional commits](https://www.conventionalcommits.org/) in mind when creating PRs, see our linter workflow for this convention: `.github/workflows/lint-conventional-prs.yml`.
- PR Title format: `<type>: <title>`, where title is one sentence explaining the reason for introducing the changeset.
- Ask what type to use when creating a PR: `deps`, `chore`, `docs`, `feat`, `fix`, `refactor`, `test`.
- The PR description should contain a short summary of the changes and if possible a reference issue ideally with the "closes" or "resolves" keywords. 
- Never mention Claude or any AI agent in commits or PRs (no author attribution, no Co-Authored-By, no references in commit messages).
