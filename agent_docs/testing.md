# Testing

## Test types

| Type | Location | Runner |
|---|---|---|
| Unit tests | `internal/`, `pkg/`, `api/` | `go test` (no envtest) |
| Integration tests | `tests/integration/controller/<name>/` | envtest + Ginkgo v2 |
| E2E tests | `.github/workflows/test-e2e.yml` | real clusters (CI only) |

## Running tests

**All tests (unit + integration):**
```sh
cd lifecycle-manager
make test
```

This also runs `make generate`, `make manifests`, `make fmt`, `make vet` first — the full
pre-flight. Use it before opening a PR.

**Unit tests only (fast, no envtest):**
```sh
make unittest-klm       # main module
make unittest-api       # api/ sub-module
make unittest-maintenancewindows
```

**A single controller's integration tests:**
```sh
cd lifecycle-manager
KUBEBUILDER_ASSETS=$(./bin/setup-envtest use 1.32.0 -p path) \
  go test ./tests/integration/controller/kyma/... -v
```

Replace `kyma` with `manifest`, `watcher`, `modulereleasemeta`, or `moduletemplate` for other
controllers.

Add `-ginkgo.focus "some test description"` to run a specific `It` block.

## envtest setup

Each controller suite has a `suite_test.go` that bootstraps envtest. The pattern is:

```go
kcpEnv = &envtest.Environment{
    CRDDirectoryPaths: []string{
        filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases"),
    },
    CRDs: externalCRDs, // cert-manager, istio loaded from config/samples/tests/crds/
}
restCfg, err = kcpEnv.Start()
```

`integration.GetProjectRoot()` walks up from the test binary until it finds `go.mod`, so tests
can be run from any working directory.

External CRD snapshots live in `config/samples/tests/crds/`:
- `cert-manager-v1.10.1.crds.yaml`
- `istio-v1.17.1.crds.yaml`

Do not update these snapshots without updating the corresponding import in the suite.

## Wiring controllers in tests

Integration tests wire up the full controller using the same composer functions as production:

```go
err = (&kyma.Reconciler{
    Client:            kcpClient,
    SkrContextFactory: testSkrContextFactory, // DualClusterFactory — starts a second envtest for SKR
    // ...
}).SetupWithManager(mgr, ...)
```

`DualClusterFactory` (`tests/integration/commontestutils/skrcontextimpl/`) spins up a second
envtest environment to simulate the SKR cluster. It replaces the real remote client; no actual
remote cluster is needed.

## Ginkgo conventions

- Each suite file: `package <controller>_test` (external test package).
- Suite bootstrap: `TestAPIs(t *testing.T)` calls `RunSpecs`.
- Short requeue intervals in tests: `Success: 1s, Busy/Error/Warning: 100ms` to keep tests fast.
- `Eventually(func, Timeout, Interval)` from `pkg/testutils` for async assertions.
- `Timeout` and `Interval` constants are defined in `pkg/testutils/` and imported via
  `. "github.com/kyma-project/lifecycle-manager/pkg/testutils"`.

## Tool versions

envtest K8s version is read from `versions.yaml`:

```yaml
envtest_k8s: "1.32.0"
envtest: "0.21"
```

`make envtest` downloads `setup-envtest` at the version in `versions.yaml`. The binary is cached
in `bin/`. You only need to run this once after a fresh checkout.
