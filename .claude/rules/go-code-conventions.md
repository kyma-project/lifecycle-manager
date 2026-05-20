---
paths:
  - "**/*.go"
---

## Code Conventions

Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/) as a baseline.

Project-specific rules enforced by `golangci-lint` (see `.golangci.yaml`):
- **Import aliases**: strictly enforced — key ones: `apimetav1`, `apicorev1`, `apierrors`, `machineryruntime`, `ctrl`, `ctrlruntime`
- **Import ordering** (gci): standard → third-party → project (`github.com/kyma-project/lifecycle-manager`) → blank → dot
- **Line length**: 120 chars | **Function length**: 80 lines | **Cyclomatic complexity**: 20
- **All linters enabled by default** — check `.golangci.yaml` before adding `//nolint`
- **`//nolint` requires explanation**: e.g., `//nolint:funlen // composition root wiring`
