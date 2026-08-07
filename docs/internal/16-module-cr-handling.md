# Module CR Handling

**Status:** Research complete. Bug Fix and Refactoring items are ready for implementation tickets. Design Decision items require team decisions — see [Open Questions](#open-questions).

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Terminology](#terminology)
- [Accepted Requirements](#accepted-requirements)
- [How It Works Today](#how-it-works-today)
  - [Install Path](#install-path)
  - [Deletion Path](#deletion-path)
  - [Listing across CRD Versions](#listing-across-crd-versions)
  - [Cluster-Scoped CR Handling](#cluster-scoped-cr-handling)
  - [Call Sites](#call-sites)
- [Problems Found](#problems-found)
  - [Correctness Bugs](#correctness-bugs)
  - [Architectural Gaps](#architectural-gaps)
- [Proposed Changes](#proposed-changes)
  - [Bug Fixes](#bug-fixes)
  - [Refactoring](#refactoring)
  - [Design Decisions](#design-decisions)
- [Open Questions](#open-questions)
- [Implementation History](#implementation-history)
- [References](#references)

---

## Summary

Module CRs are the Custom Resources that module teams expose for users to configure their modules on SAP BTP, Kyma Runtime (SKR) clusters. Lifecycle Manager (KLM) creates, tracks, and deletes a Default Module CR per module, and gates module removal on the absence of all Module CRs of the same API type.

This document captures the outcome of research spike [#3029](https://github.com/kyma-project/lifecycle-manager/issues/3029) into making Module CR handling more robust. It documents the accepted requirements, the current implementation behavior end-to-end, the correctness bugs and architectural gaps found during the investigation, and a proposed set of follow-up work items.

Key findings:

- A **parser-level namespace pollution bug** writes a namespaced value onto cluster-scoped Module CR data. The bug is partially mitigated in the deletion path. In the install path, controller-runtime omits the namespace from HTTP URL paths for cluster-scoped resources, but the `metadata.namespace` field is present in the Create request body; whether the Kubernetes API server strips or rejects it has not been empirically verified.
- A **silent error swallow** in `SyncDefaultModuleCR` prevents non-transient install failures from surfacing in the Manifest status.
- A **stuck-finalizer bug** causes the Manifest to be permanently blocked when the Default Module CR is manually deleted before the module is unmanaged.
- The **CRD version-drop contract** with module operators is undefined, blocking safe API version removal.
- The `modulecr` package violates ADRs 001–005 in four distinct ways.

---

## Motivation

Three production bugs ([#3005](https://github.com/kyma-project/lifecycle-manager/issues/3005), [#3006](https://github.com/kyma-project/lifecycle-manager/issues/3006), [#3007](https://github.com/kyma-project/lifecycle-manager/issues/3007)) and growing adoption of cluster-scoped and multi-version Module CRDs exposed that Module CR handling has grown organically without a unified specification. Each fix was applied locally without a holistic review of the full data flow, leaving partial mitigations that cover only some code paths.

Without this research, the following failure modes remain active:

- Module operators using cluster-scoped CRDs have a spurious namespace written into `Manifest.Spec.Resource` at parse time. Controller-runtime omits this from HTTP URL paths, but the value is present in the Create request body. Whether the Kubernetes API server strips or rejects that field, and whether the Default CR on the SKR is created with or without a namespace, has not been empirically verified.
- A permanent RBAC misconfiguration or transient API failure during Default CR creation returns no error to the caller. The Manifest never enters `StateError`; the failure loops silently until manually investigated.
- A user who manually deletes the Default CR before unmanaging a module leaves the Manifest stuck indefinitely with `LabelRemovalFinalizer` never removed.
- There is no safe code path for a module team to drop an old API version from their CRD without risking orphaned instances in `status.storedVersions`.

---

## Goals

This spike covers:

- Tracing all accepted requirements for Module CR handling (community#972, community#982) to their implementation with code references.
- Mapping the current implementation end-to-end — install path, three-path deletion flow, cluster-scoped handling, and cross-version listing.
- Identifying all correctness bugs with exact failure conditions and file:line evidence.
- Identifying all architectural gaps relative to KLM's ADRs 001–005.
- Proposing a concrete follow-up EPIC separated into items ready to implement and items requiring a team decision first.

---

## Non-Goals

- **Implementing any changes.** All proposed changes are follow-up tickets, not part of this spike.
- **Making decisions on open questions.** Cluster-scope signal authority, version-drop contract design, and the two-phase delete state machine require team alignment and are explicitly deferred.
- **Mandatory module handling.** Mandatory modules have no Default Module CR and follow a different code path.
- **Community module handling.** Community modules are not installed by KLM; the Default CR concept applies only to regular Kyma modules.
- **Market-specific Module CRs.** Multiple Default CRs per market are not supported and out of scope.
- **Module instance business status reporting.** KLM reports installation state, not the business status of a Module instance. This is an accepted non-requirement per community#972 (R2).

---

## Terminology

- **Module CRD** — the Custom Resource Definition provided by a module (for example, `Btp`, `Telemetry`, `Nats`). Deployed on the SKR as part of module installation. Its shape is owned by the module team, not by KLM.
- **Module CR** — any Custom Resource of the Module CRD's `GroupKind` on the SKR. Can be created by KLM, by the customer, by a UI or CLI flow, or by another operator.
- **Default Module CR** — the specific Module CR whose name, namespace, and GVK match `Manifest.Spec.Resource`. This value is derived from `ModuleTemplate.Spec.Data` during Kyma-to-Manifest translation. There is at most one Default Module CR per Manifest.
- **CustomResourcePolicy (CRP)** — a field on `Manifest.Spec` with two values:
  - `CreateAndDelete` — KLM creates the Default Module CR on install and marks it for deletion on module removal.
  - `Ignore` — KLM neither creates nor deletes the Default Module CR. `Manifest.Spec.Resource` is still populated so KLM knows the GVK/GroupKind to watch for the deletion gate.
  - Defined at [api/v1beta2/kyma_types.go:107-120](../../api/v1beta2/kyma_types.go#L107-L120).
- **KCP / SKR** — Kyma Control Plane cluster / SAP BTP, Kyma Runtime cluster.

---

## Accepted Requirements

The following requirements come from community decisions and are already implemented. They are included here for traceability — each requirement maps to the code that implements it.

### R1 — Deletion gate uses all Module CRs of the GroupKind

Both `CreateAndDelete` and `Ignore` treat every Module CR of the Module CRD's `GroupKind` across all namespaces as a gate before module deletion proceeds.

- Accepted 2025-03-12 in [community#972](https://github.com/kyma-project/community/issues/972).
- Implemented by [PR #2543](https://github.com/kyma-project/lifecycle-manager/pull/2543) (block deletion until Module CRs removed) and [PR #3012](https://github.com/kyma-project/lifecycle-manager/pull/3012) (list all namespaces).
- Code: [`CheckModuleCRsDeletion`](../../internal/manifest/modulecr/client.go#L82) and [`CheckDefaultCRDeletion`](../../internal/manifest/modulecr/client.go#L63).

### R2 — KLM does not report Module instance business status

KLM tracks installation state, not the business status of a Module instance. The Module Operator reflects business status in Module CRs — KLM does not read or propagate that status. If misconfigured customer CRs exist alongside the Default CR, this is a concern for the UI or CLI, not for KLM's status reporting.

- Accepted 2025-03-12 in community#972.
- `Kyma.Status.Modules[].Resource` exposes the tracked Default CR via [`ModuleStatusGenerator.GenerateModuleStatus`](../../internal/service/kyma/status/modules/generator/generator.go#L83-L100), but no configuration-level status is reported.

### R3 — KLM only marks the Default Module CR for deletion

For `CRP: CreateAndDelete`, customer-created Module CRs are not touched by KLM. KLM waits until they are gone (per R1) but does not delete them.

- Accepted 2025-04-23 addendum to community#972.
- Implemented at [`deleteCR`](../../internal/manifest/modulecr/client.go#L256-L292) — only the CR matched by `isResourceTheDefaultCR` is deleted.

### R4 — The Default Module CR is created at most once per manage cycle

Once KLM creates the Default Module CR, it does not re-create or re-sync it. Users or module operators can then evolve or delete it; KLM does not restore it. If the module is unmanaged and re-managed, the Manifest is deleted and recreated, resetting the condition; the Default Module CR reconciliation runs once again.

- Implemented via the `ModuleCR` Manifest condition in [PR #3126](https://github.com/kyma-project/lifecycle-manager/pull/3126), closing [#3007](https://github.com/kyma-project/lifecycle-manager/issues/3007).
- Code: [`Reconciler.syncDefaultModuleCR`](../../internal/controller/manifest/controller.go#L601-L611) gated by `ShouldCreateDefaultModuleCR() && !IsModuleCRInstallConditionTrue()`; condition defined at [`internal/manifest/status/condition.go:15`](../../internal/manifest/status/condition.go#L15).

### R5 — CRD upgrades are applied verbatim from the module manifest

KLM applies CRD changes as delivered in the module manifest. Adding a new version is safe. Dropping a version is the module team's responsibility — see [G4](#g4-dropping-a-crd-version-safely).

- Related: [ADR 006 — CRD Upgrade Strategy in Managed Mode](adr/006-crd-upgrade-strategy.md) covers KCP↔SKR CRD synchronization for KLM's own CRDs but does not cover Module CRDs specifically.

### R6 — All regular modules must provide Default Module CR data

All non-mandatory, non-community modules must include Default Module CR data in `ModuleTemplate.Spec.Data`. Consumers can expect this data to be present but must be robust to its absence at runtime.

- Accepted 2025-05-16 in [community#982](https://github.com/kyma-project/community/issues/982).
- The module publishing pipeline enforces this requirement. KLM does not validate `Spec.Data` at reconcile time.
- Current gap: KLM is not fully robust to an absent Default CR in all code paths — see [G11](#g11-labelremovalfinalizer-permanently-stuck-if-the-default-cr-is-manually-deleted).

---

## How It Works Today

### Install Path

1. The Kyma controller runs the parser at [`internal/service/manifest/parser/template_to_module.go`](../../internal/service/manifest/parser/template_to_module.go).
2. `setNameAndNamespaceIfEmpty` at [lines 133–145](../../internal/service/manifest/parser/template_to_module.go#L133-L145) defaults `name` to the module name and `namespace` to `Parser.remoteSyncNamespace` on `ModuleTemplate.Spec.Data` if either field is empty.
3. `newManifestFromTemplate` copies `template.Spec.Data` into `Manifest.Spec.Resource` at [line 160](../../internal/service/manifest/parser/template_to_module.go#L160).
4. The Manifest controller reconciles. When `ShouldCreateDefaultModuleCR() == true` and the `ModuleCR` condition is not yet `True`, it calls `SyncDefaultModuleCR` at [controller.go:606](../../internal/controller/manifest/controller.go#L606).
5. `SyncDefaultModuleCR` does a `Get` on the CR; if the CR is not found, it calls `Create` using `Manifest.Spec.Resource.DeepCopy()` as-is. See [client.go:120–137](../../internal/manifest/modulecr/client.go#L120-L137).
6. On success, `SetModuleCRInstallConditionTrue` is called at [controller.go:610](../../internal/controller/manifest/controller.go#L610). Subsequent reconciliations short-circuit on the condition.

**Invariants preserved by the install path:**

- `Manifest.Spec.Resource` is always populated when `ModuleTemplate.Spec.Data` is present, regardless of CRP. This gives KLM the GVK/GroupKind to watch for the deletion gate even under `CRP: Ignore`.
- For `CRP: Ignore`, `SyncDefaultModuleCR` at [client.go:120](../../internal/manifest/modulecr/client.go#L120) short-circuits immediately — KLM never calls `Create`.
- The Default Module CR is never listed in `Manifest.Status.Synced`. It is created outside the render/prune pipeline and appears in `Kyma.Status.Modules[].Resource` as a `TrackingObject`, not in the Manifest status.

### Deletion Path

Entry point: `Reconciler.delete` at [controller.go:342](../../internal/controller/manifest/controller.go#L342). Dispatched when `manifest.GetDeletionTimestamp().IsZero() == false`.

[`renderResourcesForDelete`](../../internal/controller/manifest/controller.go#L549) calls `ensureModuleCRsAllDeleted` at the top of every delete-path reconcile. This runs two separate list traversals back-to-back ([controller.go:582](../../internal/controller/manifest/controller.go#L582), [controller.go:586](../../internal/controller/manifest/controller.go#L586)):

1. `CheckModuleCRsDeletion` → `GetAllModuleCRsExcludingDefaultCR` → `listResourcesByGroupKindInAllNamespaces`
2. `CheckDefaultCRDeletion` → `listResourcesByGroupKindInAllNamespaces`

`renderResourcesForDelete` then produces one of three paths:

**Path A — all CRs gone (`allModuleCRsDeleted == true`):**

`renderResourcesForDelete` returns an empty target `[]client.Object{}`. The remainder of `delete()` proceeds:

- `pruneDiff(current, empty_target)` — the difference of current versus empty equals every resource in `Status.Synced`. All are sent for deletion. Operator-managed resources are deleted first; then operator-related resources, including the CRD. CRD deletion is blocked until all non-operator objects return 404.
- `RemoveDefaultModuleCR` — if the CRD is gone, `listResourcesByGroupKindInAllNamespaces` returns a `*meta.NoKindMatchError`, which `util.IsNotFound` catches; returns `(true, nil)`. Removes the CR finalizer from the Manifest via the KCP client.
- `SyncResources(empty_target)` — sets `Status.Synced = []`. Detects a diff from the old non-empty `Synced`; returns `ErrWarningResourceSyncStateDiff`, sets state to `StateDeleting`, and requeues.
- On the next reconcile: `Synced` is empty, no diff is detected, `updateDeletingState` runs, and `cleanupManifest` removes the Manifest's remaining finalizers. The Manifest is deleted.

**Path B — customer CRs gone, Default CR still present (`allModuleCRsDeleted == false, err == nil`):**

`renderResourcesForDelete` renders the full module resource target (CRD, operator, and so on) and returns it. The remainder of `delete()`:

- `pruneDiff(current, full_target)` — the CRD is in both sets, so it is not in the diff. Module resources are preserved.
- `RemoveDefaultModuleCR` — marks the Default CR for deletion with background propagation.
- `SyncResources(full_target)` — re-applies all module resources to the SKR. This keeps the module operator running so it can process the Default CR deletion.
- `updateDeletingState` — sets state to `StateDeleting`.
- On the next reconcile, once the Default CR is gone, Path A fires.

**Path C — customer CRs still exist (`ErrWaitingForModuleCRsDeletion`):**

`renderResourcesForDelete` sets the Manifest state to `StateDeleting` with the operation message "waiting for module crs deletion" and returns the error. `delete()` calls `r.finishReconcile(...)`, which persists the status and returns the error; controller-runtime requeues via exponential backoff.

**Two-phase delete model alignment ([discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442), [#833](https://github.com/kyma-project/lifecycle-manager/issues/833)):**

Path B maps to Phase 1 ("await-for-default-cr-removal"): module resources remain active on the SKR so that the module operator can process the Default Module CR's deletion — `SyncResources(full_target)` re-applies them every reconcile. Path A maps to Phase 2 ("deprovision-resources"): once all CRs are gone, module resources are removed via `pruneDiff(empty_target)`.

Per the decision from [#833](https://github.com/kyma-project/lifecycle-manager/issues/833), KLM keeps module resources (RBAC, CRDs, module operator) up to date during Phase 1. If a user re-enables a module while deletion is in progress, KLM continues the in-progress deletion to completion before starting a fresh installation.

**Invariants preserved by the deletion path:**

- The Default Module CR is NOT in `Status.Synced`, so `pruneDiff` never deletes it. `RemoveDefaultModuleCR` is the only code path that deletes it.
- `deleteCR` at [client.go:256](../../internal/manifest/modulecr/client.go#L256) short-circuits for `CRP: Ignore` — KLM never deletes the Default CR under this policy.
- KLM interacts with Module CRs via the SKR client only. The KCP client is used only for Manifest finalizer removal.
- The deletion gate (`ensureModuleCRsAllDeleted`) always runs before `pruneDiff` and before `RemoveDefaultModuleCR`.

### Listing across CRD Versions

Introduced by [PR #3021](https://github.com/kyma-project/lifecycle-manager/pull/3021) after [#3005](https://github.com/kyma-project/lifecycle-manager/issues/3005).

[`listResourcesByGroupKindInAllNamespaces`](../../internal/manifest/modulecr/client.go#L178) enumerates every version registered by `RESTMapper` for the `GroupKind` and issues one cross-namespace `List` per version, concatenating results. If a per-version `List` call fails with an error that is not NotFound-class, the error is silently swallowed with `continue` at [client.go:196](../../internal/manifest/modulecr/client.go#L196).

**Rationale:** after an in-place API version upgrade, the Manifest reference points at the new version, but the SKR may still hold instances of the old version. The list must not miss them. This was the root cause of the Huskies incident ([#3005](https://github.com/kyma-project/lifecycle-manager/issues/3005)).

### Cluster-Scoped CR Handling

Introduced by [PR #3046](https://github.com/kyma-project/lifecycle-manager/pull/3046).

[`isResourceTheDefaultCR`](../../internal/manifest/modulecr/client.go#L217-L236) treats namespaces as matching when either the found resource has an empty `metadata.namespace` or the RESTMapper reports the CRD as cluster-scoped (`meta.RESTScopeNameRoot`). This masks — for the deletion and filter path only — the namespace pollution introduced by `setNameAndNamespaceIfEmpty` in the parser.

[`ModuleStatusGenerator.GenerateModuleStatus`](../../internal/service/kyma/status/modules/generator/generator.go#L83-L100) separately masks the polluted namespace in the Kyma status by blanking it out when `ModuleTemplate` carries the annotation `operator.kyma-project.io/is-cluster-scoped` ([`shared.IsClusterScopedAnnotation`](../../api/shared/operator_annotations.go#L12)).

The install path (`GetDefaultCR` and `SyncDefaultModuleCR`) has no equivalent mitigation — see [G1](#g1-namespace-pollution-for-cluster-scoped-module-crs).

### Call Sites

Every production call site constructs `modulecr.NewClient(skrClient)` inline; there is no long-lived field and no interface abstraction.

| Site | Method |
|---|---|
| [`labelsremoval/labels_removal.go:80`](../../internal/manifest/labelsremoval/labels_removal.go#L80) | `GetDefaultCR` |
| [`controller/manifest/controller.go:374`](../../internal/controller/manifest/controller.go#L374) | `RemoveDefaultModuleCR` |
| [`controller/manifest/controller.go:582`](../../internal/controller/manifest/controller.go#L582) | `CheckModuleCRsDeletion` |
| [`controller/manifest/controller.go:586`](../../internal/controller/manifest/controller.go#L586) | `CheckDefaultCRDeletion` |
| [`controller/manifest/controller.go:606`](../../internal/controller/manifest/controller.go#L606) | `SyncDefaultModuleCR` |

**Related direct-access site (bypasses the client):**
[`purge/controller.go:265-289`](../../internal/controller/purge/controller.go#L265) — `getAllRemainingCRs` lists CRs of any CRD and strips finalizers as part of the SKR-deprovision purge. This can affect Module CRs when a Kyma is being deprovisioned past the grace period.

---

## Problems Found

### Correctness Bugs

#### G1. Namespace pollution for cluster-scoped Module CRs

`setNameAndNamespaceIfEmpty` at [template_to_module.go:141–144](../../internal/service/manifest/parser/template_to_module.go#L141-L144) unconditionally writes `remoteSyncNamespace` onto `ModuleTemplate.Spec.Data` if the namespace field is empty, regardless of whether the module's CRD is cluster-scoped. This value flows into `Manifest.Spec.Resource.Namespace`.

The bug is **partially mitigated** but the mitigations are inconsistent across three signals:

| Signal | Where used | What it mitigates |
|---|---|---|
| `operator.kyma-project.io/is-cluster-scoped` annotation on `ModuleTemplate` | Status generator only ([generator.go:83](../../internal/service/kyma/status/modules/generator/generator.go#L83)) | Post-hoc namespace masking in `Kyma.Status` |
| RESTMapper `meta.RESTScopeNameRoot` | `isResourceTheDefaultCR` ([client.go:226](../../internal/manifest/modulecr/client.go#L226)) | Deletion and filter path only |
| Empty-namespace observation on a returned resource | Same `isResourceTheDefaultCR` | Deletion and filter path only |

Neither `GetDefaultCR` at [client.go:56](../../internal/manifest/modulecr/client.go#L56) nor `SyncDefaultModuleCR` at [client.go:130](../../internal/manifest/modulecr/client.go#L130) contains an application-level cluster-scope check. Controller-runtime's `NamespaceIfScoped` omits the namespace from HTTP URL paths for cluster-scoped resources, so the GET and Create requests reach the API server at the correct cluster-scoped endpoint. For `Create`, however, the `resource` object is passed with its `metadata.namespace` field set to the polluted value; that field is present in the HTTP request body. Whether the Kubernetes API server strips or rejects a non-empty `metadata.namespace` in the body of a cluster-scoped Create has not been empirically verified.

Team decision required: which signal is authoritative and where to fix — at the parser layer, the client layer, or both. See Design Decision 1 below.

#### G2. `IsNotFound` broad aggregation and ambiguous error semantics

`util.IsNotFound` ([pkg/util/error.go:32](../../pkg/util/error.go#L32)) aggregates five distinct conditions: `machineryruntime.IsNotRegisteredError`, `meta.IsNoMatchError` (CRD absent), `apierrors.IsNotFound` (HTTP 404), `discovery.ErrGroupDiscoveryFailed` containing a 404, and two string-based fallbacks ("failed to get restmapping", "could not find the requested resource"). The string fallbacks are fragile: a transient error whose message happens to contain those substrings would be classified as "not found" and silently treated as "no CRs exist."

The intended semantics differ per call site. For the three `util.IsNotFound` checks on `listResourcesByGroupKindInAllNamespaces` results ([client.go:73](../../internal/manifest/modulecr/client.go#L73), [client.go:85](../../internal/manifest/modulecr/client.go#L85), [client.go:264](../../internal/manifest/modulecr/client.go#L264)), the intent is "CRD absent means no CRs exist" — semantically intentional. But a RESTMapper returning "failed to get restmapping" for a transient reason would produce the same result.

#### G3. Version-agnostic actions and missing installed-version tracking

The current strategy for cross-version safety is to iterate every version reported by the SKR RESTMapper. Two alternatives have been raised:

- Read the CRD from the SKR and pick the storage version or served versions explicitly.
- Persist the installed version in `Manifest.Status` and query that version directly.

Today `shared.Status` has no `InstalledVersion` field ([`api/shared/status.go:11-30`](../../api/shared/status.go#L11-L30)). Choice deferred — see Design Decision 6 below.

#### G4. Dropping a CRD version safely

Kubernetes forbids removing a version from `spec.versions` while it still appears in `status.storedVersions`. See [#2807](https://github.com/kyma-project/lifecycle-manager/issues/2807). KLM already implements [`DropStoredVersion`](../../internal/crd/storage_version_dropper.go) for KCP CRDs. No equivalent runs on SKRs today for Module CRDs.

KLM cannot know whether the module operator has migrated all CR instances on a given SKR. An explicit contract with module operators is required. Three candidate designs exist — see [discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442):

- **Option A — Per-instance annotation (primary contract):** The module operator stamps `operator.kyma-project.io/current-storage-version: "<new-version>"` on every CR instance after migration. KLM verifies all instances carry this annotation before calling `DropStoredVersion`.
- **Option B — CRD-level annotation (secondary contract):** The module operator adds `operator.kyma-project.io/dropping-storage-version: "<old-version>"` to the CRD itself as an explicit "migration complete, drop this version" signal. Lower verification granularity but simpler to implement.
- **Option C — No KLM-managed dropping:** Module teams handle version drops through their own means. KLM documents the constraint. Options A and B can be combined: B as the intent gate, A as the per-instance verification. The choice is deferred — see Design Decision 2 below.

#### G5. Redundant list traversals and distributed CRP branching in the deletion gate

`CheckModuleCRsDeletion` and `CheckDefaultCRDeletion` each invoke `listResourcesByGroupKindInAllNamespaces` on the same `GroupKind` back-to-back with no result sharing.

The CRP logic is split across both functions:

- `CheckDefaultCRDeletion` short-circuits immediately for `CRP: Ignore` at [client.go:67](../../internal/manifest/modulecr/client.go#L67).
- `GetAllModuleCRsExcludingDefaultCR` (called by `CheckModuleCRsDeletion`) includes all CRs — not filtering out the Default CR — for `CRP: Ignore` at [client.go:157-162](../../internal/manifest/modulecr/client.go#L157-L162).

The two branches cancel each other out; the combined gate is R1-compliant. However, the distributed branching is opaque and risks regressions in future edits. The two checks can be collapsed into a single CRP-independent "any CR of the GroupKind exists?" function. See Refactoring 5 below.

#### G6. Renamed or moved Default Module CR

If a module team changes `ModuleTemplate.Spec.Data.metadata.name` or `metadata.namespace` between versions, the Manifest's new `Spec.Resource` no longer points at the previously created CR. `SyncDefaultModuleCR` would create a new CR alongside the old one. `RemoveDefaultModuleCR` would target only the new one; the old CR becomes a "customer CR" from KLM's perspective and blocks the deletion gate until it is removed manually. No error is produced — the user only sees `StateDeleting` indefinitely.

Options: reject the change via admission validation, support the rename explicitly with previous-name tracking, or document it as a known limitation for module teams. See Design Decision 3 below.

#### G11. `LabelRemovalFinalizer` permanently stuck if the Default CR is manually deleted

`removeFromDefaultCR` at [labels_removal.go:72–86](../../internal/manifest/labelsremoval/labels_removal.go#L72) has no NotFound tolerance:

```go
defaultCR, err := modulecr.NewClient(skrClient).GetDefaultCR(ctx, manifest)
if err != nil {
    return fmt.Errorf("failed to get default CR, %w", err)
    // no util.IsNotFound check — any error, including 404, propagates
}
```

If the Default CR is manually deleted before the module is unmanaged, `GetDefaultCR` returns a wrapped NotFound error. `RemoveManagedByLabel` receives that error and returns it without removing `LabelRemovalFinalizer`. The Manifest is stuck indefinitely; every subsequent reconcile repeats the same failure. See Bug Fix 2 below.

#### G12. `SyncDefaultModuleCR` silently swallows non-NotFound errors from `Get`

```go
// client.go:126
if err := c.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil && util.IsNotFound(err) {
```

When `Get` returns a non-NotFound error (transient API failure, network timeout, RBAC denial), the condition evaluates to `false`. The create block is skipped, the function returns `nil`, and the error is never surfaced. The `ModuleCR` condition remains `False`; subsequent reconciles retry silently. Persistent failures — for example, a permanent RBAC misconfiguration — loop indefinitely without the Manifest ever entering `StateError`. See Bug Fix 3 below.

#### G13. Shared Module CRD between two modules

`listResourcesByGroupKindInAllNamespaces` queries by `GroupKind` only, with no filtering by owner, Manifest, or label. If two modules ship the same `GroupKind`, each Manifest's deletion gate blocks on the other's CRs. Which Manifest unblocks first depends on the order in which CRs are deleted, making the outcome non-deterministic. No error or warning is produced. [community#982](https://github.com/kyma-project/community/issues/982) assumes each Default CR belongs exclusively to one module, but the current code cannot enforce this.

---

### Architectural Gaps

#### G7. No consumer-defined interface (ADR 001)

Callers depend on the concrete `*modulecr.Client` struct. No interface exists at the consumer side and no mock is generated. Test coverage relies on the real client against a fake `client.Client`.

#### G8. Inline construction violates composition-root dependency injection (ADR 002)

`NewClient` is called on the fly in five production sites. The modulecr client itself is not composed at `cmd/main.go`. A single service instance constructed once at the composition root would let callers receive it via constructor injection.

#### G9. Mixed layers violate layered architecture (ADR 004)

The package sits at `internal/manifest/modulecr/`, not under `service/` or `repository/`. Its methods mix service-level orchestration (`SyncDefaultModuleCR`, `RemoveDefaultModuleCR`) with repository-level access (`listResourcesByGroupKindInAllNamespaces`, `GetDefaultCR`). Splitting into a `ModuleCRService` and a `ModuleCRRepository` would align with the layered architecture.

#### G10. Misleading names and unexported dead surface (ADR 005)

- `SyncDefaultModuleCR` is misleading — it is create-if-missing, never sync. Consider `EnsureDefaultCRCreated`.
- `GetAllModuleCRsExcludingDefaultCR` is public but has no callers outside the `internal/manifest/modulecr/` package. It is called only from `CheckModuleCRsDeletion` internally and directly from tests.
- `Client` in a package named `modulecr` violates the "let the package provide the context" rule — it should be `ModuleCRService` or `ModuleCRRepository` per ADR 005.

---

## Proposed Changes

### Bug Fixes

Items in this group address confirmed, code-verified broken behavior. Each maps to a `kind/bug` issue.

1. Fix `setNameAndNamespaceIfEmpty` at [template_to_module.go:141–144](../../internal/service/manifest/parser/template_to_module.go#L141-L144) to not write `remoteSyncNamespace` onto cluster-scoped Module CR data. The parser unconditionally stamps the namespace field regardless of CRD scope — confirmed from code. As part of this fix, verify empirically whether the Kubernetes API server strips or retains the `metadata.namespace` field in the Create request body for cluster-scoped resources. Blocked on Design Decision 1 (cluster-scope signal). (G1)
2. Fix `removeFromDefaultCR` to tolerate `NotFound` on `GetDefaultCR` — if the Default CR is already gone, the label-removal step must succeed rather than leaving `LabelRemovalFinalizer` stuck. (G11)
3. Fix `SyncDefaultModuleCR` to return non-`NotFound` `Get` errors: change the condition at [client.go:126](../../internal/manifest/modulecr/client.go#L126) so that any error that is not NotFound-class is returned rather than silently discarded. (G12)

### Refactoring

Items in this group align the package with ADRs 001–005 without changing observable behavior. Each maps to a `kind/cleanup` issue.

1. Extract a consumer-defined interface (`ModuleCRService` or narrower) at each caller; make callers depend on interfaces, not the concrete struct. (G7, ADR 001)
2. Split into `internal/service/manifest/modulecr` (orchestration) and `internal/repository/manifest/modulecr` (Kubernetes I/O). (G9, ADR 004)
3. Wire the service once at composition root; remove the five inline `NewClient` sites. (G8, ADR 002)
4. Rename `SyncDefaultModuleCR` to `EnsureDefaultCRCreated`; unexport or remove `GetAllModuleCRsExcludingDefaultCR`. (G10, ADR 005)
5. Consolidate `CheckDefaultCRDeletion` and `CheckModuleCRsDeletion` into a single CRP-independent "any CR of the GroupKind exists?" gate, eliminating the duplicate list traversal and the distributed CRP branching. The current behavior is R1-compliant; this is a readability and regression-safety improvement. (G5)
6. Clarify the expected error semantics per `util.IsNotFound` call site; distinguish "CRD absent, treat as no CRs exist" from "transient API failure, surface as error". The current string-based fallbacks are structurally fragile but no production misclassification has been confirmed. (G2)

### Design Decisions

Items in this group require an explicit team decision before implementation can begin. Each is a candidate for an ADR and maps to a `goal/architecture` issue.

1. Decide the authoritative cluster-scope signal — annotation, RESTMapper, or both — and where to enforce it. (G1)
2. Define the "safe to drop version" contract with module operators. Write the spec (Options A, B, or C from G4), then implement. ([#2807](https://github.com/kyma-project/lifecycle-manager/issues/2807), [#2905](https://github.com/kyma-project/lifecycle-manager/issues/2905))
3. Define behavior for a renamed or moved Default Module CR — reject via validation, support explicitly, or document as a known limitation. (G6)
4. Follow up on [#2428](https://github.com/kyma-project/lifecycle-manager/issues/2428) — evolution of `CRP: CreateAndDelete` with UI/CLI-driven module configuration.
5. Define and implement the formal two-phase delete state machine (`await-for-cr-removal` → `deprovision-resources`) as explicit Manifest states with observable transitions. ([discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442), [#833](https://github.com/kyma-project/lifecycle-manager/issues/833))
6. Decide whether to add a `Manifest.Status.InstalledVersion` field and query that version directly, or to read the CRD storage version from the SKR, rather than iterating all RESTMapper versions. The current RESTMapper-based approach is functional; this is an enhancement to query precision and auditability. (G3)

---

## Open Questions

These questions require a team decision before the corresponding Design Decision items can be implemented. They are tracked in [discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442).

1. **Cluster-scope signal authority (Design Decision 1):** Three signals currently disagree — the `is-cluster-scoped` annotation on `ModuleTemplate`, the SKR RESTMapper scope, and the empty-namespace observation. Which one is authoritative? Should the fix happen at the parser layer, the client layer, or both?

2. **Version-drop contract design (Design Decision 2):** Should KLM support dropping old CRD API versions on behalf of module operators? If yes, which contract model — per-instance annotation (Option A), CRD-level annotation (Option B), or a combination? If no, how is the constraint communicated clearly to module teams?

3. **Renamed Default CR handling (Design Decision 3):** Should KLM reject a Default CR name or namespace change via admission validation, support renames with previous-name tracking, or document the gap as a module-team responsibility?

4. **Two-phase delete formalization (Design Decision 5):** Should the current Path A/B/C behavior be formalized as an explicit Manifest state machine? If yes, what are the exact state names, transitions, and timeout semantics?

---

## Implementation History

- **2025-07-22:** Research spike [#3029](https://github.com/kyma-project/lifecycle-manager/issues/3029) completed. Document added via [PR #3453](https://github.com/kyma-project/lifecycle-manager/pull/3453). Findings shared in [discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442).

---

## References

- [community#972 — Module CR handling requirements](https://github.com/kyma-project/community/issues/972)
- [community#982 — All modules must provide Default Module CR data](https://github.com/kyma-project/community/issues/982)
- [lifecycle-manager#3005 — API-version-upgrade cleanup bug (Huskies)](https://github.com/kyma-project/lifecycle-manager/issues/3005)
- [lifecycle-manager#3006 — cross-namespace Module CR gate bug](https://github.com/kyma-project/lifecycle-manager/issues/3006)
- [lifecycle-manager#3007 — Default CR create-once](https://github.com/kyma-project/lifecycle-manager/issues/3007)
- [lifecycle-manager#2807 — CRD upgrade / storedVersions](https://github.com/kyma-project/lifecycle-manager/issues/2807)
- [lifecycle-manager#2905 — Dropping stored versions safely](https://github.com/kyma-project/lifecycle-manager/issues/2905)
- [lifecycle-manager#2428 — UX evolution for Module CR configuration](https://github.com/kyma-project/lifecycle-manager/issues/2428)
- [lifecycle-manager#833 — Module re-enable during deletion / keep resources up to date while awaiting CR deletion](https://github.com/kyma-project/lifecycle-manager/issues/833)
- [lifecycle-manager discussion #3442 — Make ModuleCR handling more robust](https://github.com/kyma-project/lifecycle-manager/discussions/3442)
- [ADR 006 — CRD Upgrade Strategy](adr/006-crd-upgrade-strategy.md)
