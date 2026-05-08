---
paths:
  - "**/*.go"
---

# Go code conventions — lifecycle-manager

`make lint` is the authoritative check. The full linter config is in `.golangci.yaml`.

## Import aliases

Strict aliases are enforced by `importas` — violations fail CI. The **complete alias list** is in `.golangci.yaml` under `linters-settings.importas.alias` (75 entries). When adding an import, check that file first.

Import ordering is enforced by `gci`: **standard → third-party → project** (`github.com/kyma-project/lifecycle-manager`) **→ blank → dot**.

## nolint policy

Every `//nolint` directive **must** include an explanation:
```go
//nolint:funlen // composition root wiring — acceptable exception
```
Bare `//nolint:funlen` fails review. Check `.golangci.yaml` before adding any suppression.

## FIPS

Use `GOFIPS140=v1.0.0 go` for any `go` command run directly (the Makefile sets this automatically). Do not add dependencies that bypass the FIPS-approved stdlib crypto (no third-party elliptic curves, no custom cipher suites).
