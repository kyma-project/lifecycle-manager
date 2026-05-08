---
paths:
  - "**/*.go"
---

# Go code conventions — lifecycle-manager

These rules are enforced by CI (`make lint`). Violations fail the build.

## Import aliases

The `importas` linter enforces strict aliases. Key ones used in almost every file:

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

Full alias list: `.golangci.yaml` → `linters-settings.importas.alias`.

## Import ordering

Enforced by `gci`: **standard → third-party → project** (`github.com/kyma-project/lifecycle-manager`) **→ blank → dot**.

## Lint limits

- Line length: **120 characters** (`revive line-length-limit`)
- Function length: **80 lines** (`funlen`) — `//nolint:funlen // <reason>` only for composition root wiring
- Cyclomatic complexity: **20** (`cyclop`)
- All linters enabled by default; check `.golangci.yaml` before adding any `//nolint`. Every directive **must** include an explanation comment.

## FIPS

Use `GOFIPS140=v1.0.0 go` for any `go` command run directly (Makefile sets this automatically). Do not add dependencies that use non-FIPS-approved crypto.
