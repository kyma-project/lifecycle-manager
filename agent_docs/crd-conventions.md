# CRD conventions

## API group

All lifecycle-manager CRDs live under `operator.kyma-project.io`.

The constant for the group is `shared.OperatorGroup = "operator.kyma-project.io"` in
`api/shared/`.

## Versions

| Version | Status |
|---|---|
| `v1beta1` | Deprecated; kept for conversion |
| `v1beta2` | Current storage version (marked `+kubebuilder:storageversion`) |

When adding a field, add it to `v1beta2`. Do not add new fields to `v1beta1`. Conversion webhooks
handle clients that still send v1beta1 objects.

## Required markers on every CRD type

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion          // on the current storage version only
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
```

`+kubebuilder:subresource:status` is mandatory for any type that has a `.status` field —
without it, `r.Status().Update()` writes to the main object instead of the status subresource
and will overwrite spec on conflict.

## Field validation markers

Use `+kubebuilder:validation:*` markers directly above the field, not on the type:

```go
// +kubebuilder:validation:Pattern:=^[a-z]+$
// +kubebuilder:validation:MaxLength:=32
// +kubebuilder:validation:MinLength:=3
Channel string `json:"channel"`
```

For optional fields with a default:

```go
// +kubebuilder:default:=CreateAndDelete
CustomResourcePolicy `json:"customResourcePolicy,omitempty"`
```

## List types

For fields that are lists keyed by a sub-field, declare the list type and key to enable
strategic merge patch:

```go
// +listType=map
// +listMapKey=name
Modules []Module `json:"modules,omitempty"`
```

## Pruning unknown fields

To preserve an opaque/dynamic field that controller-gen would otherwise strip:

```go
// +kubebuilder:pruning:PreserveUnknownFields
Source machineryruntime.RawExtension `json:"source"`
```

## Generated files — do not hand-edit

| Generated file | Source |
|---|---|
| `config/crd/bases/*.yaml` | `make manifests` |
| `config/rbac/common/*.yaml` | `make manifests` |
| `api/v1beta2/zz_generated.deepcopy.go` | `make generate` |
| `api/v1beta1/zz_generated.deepcopy.go` | `make generate` |

Editing these files by hand will be overwritten on the next `make manifests` or
`make generate` run, and the CI check will catch any drift on PRs.

## Naming conventions

- Kind: PascalCase singular (`Kyma`, `Manifest`, `ModuleTemplate`)
- Resource (plural): lowercase (`kymas`, `manifests`, `moduletemplates`)
- controller-gen derives the plural automatically; override only if needed with
  `// +kubebuilder:resource:plural=customname`
- Shared constants (labels, annotations, finalizer names) live in `api/shared/` and are used
  by both the operator and external consumers of the API module.
