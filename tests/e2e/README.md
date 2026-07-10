# E2E Tests

This directory contains the end-to-end tests for Kyma Lifecycle Manager (KLM). The tests run in CI pipelines, but you can also run them locally against k3d clusters.

## Prerequisites

You need the following tools before running the tests:

- [Docker](https://www.docker.com/)
- [Go](https://go.dev/) (see [`versions.yaml`](../../versions.yaml) for the required version)
- GNU Make ≥ 3.82 — macOS ships Make 3.81, which is not supported. Install a newer version with `brew install make` and use `gmake` instead of `make` in all commands below.
- The [`template-operator`](https://github.com/kyma-project/template-operator) repository checked out at the same level as this repository — the makefiles resolve it at `../template-operator/` relative to the `lifecycle-manager` root.

All other tools (ginkgo, kustomize, modulectl, istioctl, ocm) are installed automatically into `tests/e2e/bin/` the first time you run any test target.

## Structure

The test infrastructure uses a layered makefile design:

| File | Purpose |
|---|---|
| `Makefile` | Top-level entry point. Contains a named target for every test scenario, for example `watcher-enqueue` or `mandatory-module`. Run `make help` to list all targets. |
| `e2e.common.mk` | Shared infrastructure included by every test-specific `.mk` file. Defines tool installation, cluster lifecycle, and module setup helpers. |
| `<name>_test.mk` | Per-test makefile. Includes `e2e.common.mk` and defines the four required targets (see [Test-Specific Makefiles](#test-specific-makefiles)). |
| `e2e.no_module_setup.mk` | Shared makefile for tests that don't need module setup. Requires the `GINKGO_FOCUS` variable. |
| `e2e.standard_module_setup.mk` | Shared makefile for tests that use the standard template-operator setup. Requires the `GINKGO_FOCUS` variable. |
| `e2e_test_template.mk` | Blank template for creating a new test makefile. |
| `*_test.go` | Go test files with the actual Ginkgo test specs. |

### Test-Specific Makefiles

Every test-specific `.mk` file includes `e2e.common.mk` and must define the following targets:

| Target | Description |
|---|---|
| `test` | Full pipeline: chains `create-clusters`, `klm-patch`, `deploy-klm`, `module-setup`, and `test-run` in order. |
| `klm-patch` | Applies test-specific patches to KLM manifests before deployment. Tests that need no patches define this as a no-op. |
| `module-setup` | Deploys the test-specific ModuleTemplates and ModuleReleaseMeta objects. Tests with no module dependencies define this as a no-op. |
| `test-run` | Runs the Ginkgo test suite with the correct focus string against an already-deployed environment. |

### Shared Infrastructure from `e2e.common.mk`

`e2e.common.mk` provides the following shared targets:

| Target | Description |
|---|---|
| `create-clusters` | Creates the KCP and SKR k3d test clusters and installs all cluster dependencies. |
| `deploy-klm` | Builds and deploys KLM into the KCP cluster. Locally, it builds from source; in CI, it pulls from a registry. |
| `teardown` | Deletes the KCP and SKR test clusters. |
| `tools-install` | Installs all required tools (ginkgo, kustomize, modulectl, istioctl, ocm) into `tests/e2e/bin/`. |
| `module-setup-latest` | Deploys the latest template-operator ModuleTemplate. |
| `module-setup-in-older-version` | Deploys the older template-operator version. |
| `module-setup-in-newer-version` | Deploys the newer template-operator version. |

## Running Tests

All commands in this section are run from the `lifecycle-manager` repository root. Replace `<test-name>` with a target from the `Makefile`, and `<test>.mk` with the path to the test-specific makefile.

### Running a Full Test

The `Makefile` provides named top-level targets that delegate to the appropriate test makefile:

```bash
# From the lifecycle-manager root
make -C tests/e2e <target>
```

For example:

```bash
make -C tests/e2e mandatory-module
make -C tests/e2e watcher-enqueue
make -C tests/e2e watcher-zero-downtime
```

You can also call the test makefile directly:

```bash
make -f tests/e2e/mandatory_module_test.mk test
```

For tests that use a shared parameterized makefile, you must pass a `GINKGO_FOCUS` value:

```bash
make -f tests/e2e/e2e.standard_module_setup.mk test "GINKGO_FOCUS=Labelling SKR resources"
make -f tests/e2e/e2e.no_module_setup.mk test "GINKGO_FOCUS=RBAC Privileges"
```

### Running Individual Stages

You can run individual pipeline stages when you want to reuse existing clusters across multiple test runs or iterate quickly on the test code itself.

**Setting up clusters only** (no KLM deployment):

```bash
make -f tests/e2e/<test>.mk create-clusters
```

**Deploying KLM** (clusters must already be running):

```bash
make -f tests/e2e/<test>.mk klm-patch deploy-klm
```

**Setting up module metadata** (KLM must already be deployed):

```bash
make -f tests/e2e/<test>.mk module-setup
```

**Running the test only** (full environment must already be set up):

```bash
make -f tests/e2e/<test>.mk test-run
```

**Tearing down clusters** when you are done:

```bash
make -f tests/e2e/<test>.mk teardown
```

### Gardener CertManager Variant

Tests that validate certificate rotation behavior use Gardener CertManager (GCM) instead of cert-manager. These tests set `USE_GCM := true` in their makefile, which causes `create-clusters` to install GCM and `deploy-klm` to apply the GCM-specific kustomize overlay.

The GCM variant tests are:

- `watcher-zero-downtime` — uses `watcher_zero_downtime_gcm_test.mk`
- `watcher-enqueue` — uses `watcher_enqueue_gcm_test.mk`

You can identify GCM makefiles by the `USE_GCM := true` line near the top of the file.

## Adding a New Test

To add a new end-to-end test scenario:

1. Copy `e2e_test_template.mk` to a new file named after your scenario, for example `my_feature_test.mk`.
2. Implement the four required targets in the new file: `test`, `klm-patch`, `module-setup`, and `test-run`.
3. Add a named target to the `Makefile` that delegates to your new makefile.
4. Write the Ginkgo test specs in a new `*_test.go` file.

If your test needs no module setup or uses the standard template-operator setup, use `e2e.no_module_setup.mk` or `e2e.standard_module_setup.mk` instead of creating a dedicated makefile.
