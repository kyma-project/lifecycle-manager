# Code generation

## Two separate codegen steps

| Command | What it generates | When to run |
|---|---|---|
| `make generate` | `zz_generated.deepcopy.go` in each `api/` version package | Any change to a type struct in `api/v1beta1/` or `api/v1beta2/` |
| `make manifests` | CRD YAML in `config/crd/bases/`, RBAC in `config/rbac/common/` | Any change to a kubebuilder marker or any type in `api/` |

**After touching any type in `api/`**, run both:
```sh
cd lifecycle-manager
make generate
make manifests
```

The CI workflow `check-generated-code.yml` runs `make generate && make manifests && git diff
--exit-code` on every PR and will block merge if generated files are out of sync.

## What controller-gen reads

`make generate` invocation:
```
$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
```

`make manifests` invocation:
```
$(CONTROLLER_GEN) rbac:roleName=controller-manager crd webhook \
  paths="./..." \
  output:crd:artifacts:config=config/crd/bases \
  output:rbac:dir=config/rbac/common
```

controller-gen scans all Go files under `./...` for `// +kubebuilder:*` comments and Go type
definitions. The `api/` directory is a **separate Go module**, so controller-gen is run from the
main module root, which has `api/` as a replace directive in `go.mod` — this means it traverses
into `api/`.

## Generated file locations

```
api/v1beta1/zz_generated.deepcopy.go    ← make generate
api/v1beta2/zz_generated.deepcopy.go    ← make generate
api/shared/zz_generated.deepcopy.go     ← make generate

config/crd/bases/
  operator.kyma-project.io_kymas.yaml
  operator.kyma-project.io_manifests.yaml
  operator.kyma-project.io_moduletemplates.yaml
  operator.kyma-project.io_modulereleasemetas.yaml
  operator.kyma-project.io_watchers.yaml

config/rbac/common/
  role.yaml                              ← make manifests (RBAC markers)
```

None of these files should be edited by hand.

## Tool version

controller-gen version is pinned in `versions.yaml`:
```yaml
controllerTools: "0.18.0"
```

`make controller-gen` downloads and caches it to `bin/controller-gen`. Run this once after
a fresh checkout or when `versions.yaml` changes.

## Troubleshooting drift

If the CI check fails with a diff in a generated file:
1. Run `make generate && make manifests` locally.
2. `git diff` to confirm the generated change is as expected.
3. Commit the generated files alongside the type change.

If the diff is unexpected (e.g. a formatting-only change), check whether `controller-gen` was
upgraded in `versions.yaml` and whether `make controller-gen` needs to be re-run to pick up the
new binary.
