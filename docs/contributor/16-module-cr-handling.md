# Module CR Handling

This document describes how Lifecycle Manager (KLM) handles Module Custom Resources (Module CRs) on managed SAP BTP, Kyma Runtime clusters (SKRs). It captures the requirements and accepted decisions, the current implementation, and the gaps and open topics that still need to be resolved.

It is the outcome of the research spike tracked in [lifecycle-manager#3029](https://github.com/kyma-project/lifecycle-manager/issues/3029).

---

## 1. Scope and Terminology

- **Module CRD** — the Custom Resource Definition provided by a module (for example `Btp`, `Telemetry`, `Nats`). Deployed on the SKR as part of the module installation. Its shape is owned by the module team, not by KLM.
- **Module CR** — any Custom Resource of the Module CRD's `GroupKind` on the SKR. May be created by KLM, by the customer, by a UI/CLI flow, or by another operator.
- **Default Module CR** — the specific Module CR whose name/namespace/GVK match `Manifest.Spec.Resource`. This value is derived from `ModuleTemplate.Spec.Data` during Kyma-to-Manifest translation. There is at most one Default Module CR per Manifest.
- **CustomResourcePolicy (CRP)** — a field on `Manifest.Spec` (originally on `Kyma.Spec.Modules[].CustomResourcePolicy`) with two values:
  - `CreateAndDelete` — KLM creates the Default Module CR on install and marks it for deletion on module removal.
  - `Ignore` — KLM neither creates nor deletes the Default Module CR. `Manifest.Spec.Resource` is still populated so KLM knows the GVK/GroupKind to watch for deletion gating.
  - Defined at [api/v1beta2/kyma_types.go:107-120](../../api/v1beta2/kyma_types.go#L107-L120).
- **KCP / SKR** — Kyma Control Plane cluster / SAP BTP, Kyma Runtime cluster.

---

## 2. Requirements and Accepted Decisions

The authoritative requirements come from [kyma-project/community#972](https://github.com/kyma-project/community/issues/972). Summarized:

### R1 — Deletion gate uses ALL Module CRs of the GroupKind
Both `CreateAndDelete` and `Ignore` treat every Module CR of the Module CRD's `GroupKind` — across all namespaces — as a gate before proceeding with the module deletion.

- Accepted 2025-03-12 in community#972.
- Implemented by [PR #2543](https://github.com/kyma-project/lifecycle-manager/pull/2543) (block deletion until Module CRs removed) and [PR #3012](https://github.com/kyma-project/lifecycle-manager/pull/3012) (list all namespaces).
- Code: [`CheckModuleCRsDeletion`](../../internal/manifest/modulecr/client.go#L82) and [`CheckDefaultCRDeletion`](../../internal/manifest/modulecr/client.go#L63).

### R2 — KLM does not report module configuration status in the Kyma status
KLM tracks installation state, not module "business state". If misconfigured customer CRs exist alongside the default CR, this is a concern for the UI/CLI, not for KLM's status reporting.

- Accepted 2025-03-12 in community#972.
- Implemented: `Kyma.Status.Modules[].Resource` still exposes the tracked default CR via [`ModuleStatusGenerator.GenerateModuleStatus`](../../internal/service/kyma/status/modules/generator/generator.go#L83-L100), but no configuration-level status is reported.

### R3 — For CRP `CreateAndDelete`, KLM only marks the Default Module CR for deletion
Customer-created Module CRs are not touched by KLM. KLM waits until they are gone (per R1) but does not delete them itself.

- Accepted 2025-04-23 addendum to community#972.
- Implemented at [`deleteCR`](../../internal/manifest/modulecr/client.go#L256-L292) — only the CR matched by `isResourceTheDefaultCR` is deleted.

### R4 — The Default Module CR is created at most once per manage cycle
Once KLM creates the Default Module CR, it does not re-create or re-sync it. Users (or module operators) may then evolve or delete the Default Module CR; KLM does not restore it.

- Accepted and implemented via the `ModuleCR` Manifest condition in [PR #3126](https://github.com/kyma-project/lifecycle-manager/pull/3126), closing [#3007](https://github.com/kyma-project/lifecycle-manager/issues/3007).
- Behavior: If a module is unmanaged and re-managed, the Manifest is deleted and recreated, resetting the condition. On re-manage, the Default Module CR reconciliation runs once again as normal.
- Code: [`Reconciler.syncDefaultModuleCR`](../../internal/controller/manifest/controller.go#L601-L611) gated by `ShouldCreateDefaultModuleCR() && !IsModuleCRInstallConditionTrue()`; condition defined at [`internal/manifest/status/condition.go:15`](../../internal/manifest/status/condition.go#L15).

### R5 — CRD upgrades are applied verbatim from the module manifest
KLM applies CRD changes as delivered in the module manifest. Adding a new version is safe. Dropping a version is the module team's responsibility — see gaps G3 and G4.

- Related: [ADR 006 — CRD Upgrade Strategy in Managed Mode](adr/006-crd-upgrade-strategy.md) covers KCP↔SKR CRD synchronization for KLM's own CRDs but does not cover Module CRDs specifically.

### R6 — All regular modules must provide Default Module CR data
All non-mandatory, non-community modules must include Default Module CR data in `ModuleTemplate.Spec.Data`. Consumers can expect this data to be present but must be robust to its absence at runtime.

- Accepted 2025-05-16 in [community#982](https://github.com/kyma-project/community/issues/982).
- The module publishing pipeline enforces this requirement. KLM does not validate `Spec.Data` at reconcile time.
- Current gap: KLM is not fully robust to an absent Default CR in certain code paths — see G11.

---

## 3. Boundary Conditions

The invariants below hold in the current implementation and must be preserved by any refactor.

- **B1.** `Manifest.Spec.Resource` is always populated when the ModuleTemplate provides `Spec.Data`, regardless of CRP. This lets KLM know the GVK/GroupKind to watch for the deletion gate even under `CRP: Ignore`. See [PR #2543](https://github.com/kyma-project/lifecycle-manager/pull/2543).
- **B2.** For `CRP: Ignore`, KLM never calls `Create` or `Delete` against the Module CR. Both `SyncDefaultModuleCR` ([client.go:120](../../internal/manifest/modulecr/client.go#L120)) and `deleteCR` ([client.go:256](../../internal/manifest/modulecr/client.go#L256)) short-circuit when policy is `Ignore`.
- **B3.** The deletion gate (`ensureModuleCRsAllDeleted`) always runs before `pruneDiff` and before `RemoveDefaultModuleCR`. The delete-path has three distinct branches depending on the gate's result; see section 4.3 for the full flow. The Default Module CR is NOT in `Status.Synced` (see B4), so `pruneDiff` never deletes it; `RemoveDefaultModuleCR` is the only path that deletes it.
- **B4.** The Default Module CR is never listed in `Manifest.Status.Synced`. It is created via `SyncDefaultModuleCR` outside the render/prune pipeline. It appears in `Kyma.Status.Modules[].Resource` (a `TrackingObject`), not in the Manifest status.
- **B5.** KLM interacts with Module CRs via the SKR client only. The KCP client is used only for finalizer removal on the Manifest itself.

---

## 4. Current Architecture

### 4.1 Package layout
- Client: [`internal/manifest/modulecr/client.go`](../../internal/manifest/modulecr/client.go). Single struct `Client` embedding `client.Client`.
- Constructor: `NewClient(client.Client)`. Called inline in five production locations (see 4.3).
- Public methods: `GetDefaultCR`, `CheckDefaultCRDeletion`, `CheckModuleCRsDeletion`, `GetAllModuleCRsExcludingDefaultCR`, `SyncDefaultModuleCR`, `RemoveDefaultModuleCR`.
- Sentinel errors: `ErrNoResourceDefined`, `ErrWaitingForModuleCRsDeletion`.

### 4.2 Install / reconcile data flow
1. Kyma controller runs the parser at [`internal/service/manifest/parser/template_to_module.go`](../../internal/service/manifest/parser/template_to_module.go).
2. `setNameAndNamespaceIfEmpty` at [line 133-145](../../internal/service/manifest/parser/template_to_module.go#L133-L145) defaults `name` to the module name and `namespace` to `Parser.remoteSyncNamespace` on `ModuleTemplate.Spec.Data`.
3. `newManifestFromTemplate` copies `template.Spec.Data` into `Manifest.Spec.Resource` at [line 160](../../internal/service/manifest/parser/template_to_module.go#L160).
4. Manifest controller reconciles. When `ShouldCreateDefaultModuleCR() == true` and the `ModuleCR` condition is not yet True, it calls `SyncDefaultModuleCR` at [controller.go:606](../../internal/controller/manifest/controller.go#L606).
5. `SyncDefaultModuleCR` does a `Get` on the CR; if NotFound, `Create` using `Manifest.Spec.Resource.DeepCopy()` as-is. See [client.go:120-137](../../internal/manifest/modulecr/client.go#L120-L137).
6. On success, `SetModuleCRInstallConditionTrue` is called at [controller.go:610](../../internal/controller/manifest/controller.go#L610). Subsequent reconciliations short-circuit.

### 4.3 Deletion data flow
Entry point: `Reconciler.delete` at [controller.go:342](../../internal/controller/manifest/controller.go#L342). Dispatched when `manifest.GetDeletionTimestamp().IsZero() == false`.

[`renderResourcesForDelete`](../../internal/controller/manifest/controller.go#L549) calls `ensureModuleCRsAllDeleted` at the top of every delete-path reconcile. This runs two separate list traversals back-to-back ([controller.go:582](../../internal/controller/manifest/controller.go#L582), [controller.go:586](../../internal/controller/manifest/controller.go#L586)):
1. `CheckModuleCRsDeletion` → `GetAllModuleCRsExcludingDefaultCR` → `listResourcesByGroupKindInAllNamespaces`
2. `CheckDefaultCRDeletion` → `listResourcesByGroupKindInAllNamespaces`

`renderResourcesForDelete` then produces one of three paths:

**Path A — all CRs gone (`allModuleCRsDeleted == true`):**  
`renderResourcesForDelete` returns an empty target `[]client.Object{}`. The remainder of `delete()` proceeds:
- `pruneDiff(current, empty_target)` — `Difference` of current vs empty = every resource in `Status.Synced`. All are sent for deletion. Operator-managed resources are deleted first; then operator-related resources (including the CRD). CRD deletion is blocked until all non-operator objects return 404.
- `RemoveDefaultModuleCR` — if CRD is gone, `listResourcesByGroupKindInAllNamespaces` returns a `*meta.NoKindMatchError`, which `util.IsNotFound` catches; returns `(true, nil)`. Removes the CR finalizer from the Manifest via KCP client.
- `SyncResources(empty_target)` — sets `Status.Synced = []`. Detects diff from old non-empty Synced; returns `ErrWarningResourceSyncStateDiff`, sets state to `StateDeleting`. The manifest requeues.
- On the next reconcile: Synced is empty, no diff, `updateDeletingState` runs, `cleanupManifest` removes the Manifest's remaining finalizers. Manifest is deleted.

**Path B — customer CRs gone, default CR still present (`allModuleCRsDeleted == false, err == nil`):**  
`renderResourcesForDelete` renders the full module resource target (CRD, operator, etc.) and returns it. The remainder of `delete()`:
- `pruneDiff(current, full_target)` — CRD is in both, so it is NOT in the diff. Module resources are preserved.
- `RemoveDefaultModuleCR` — marks the default CR for deletion (background propagation).
- `SyncResources(full_target)` — re-applies all module resources to the SKR (keeps the module operator running so it can process the default CR deletion).
- `updateDeletingState` — sets state to `StateDeleting`.
- On the next reconcile, once the default CR is gone, Path A fires.

**Path C — customer CRs still exist (`ErrWaitingForModuleCRsDeletion`):**  
`renderResourcesForDelete` sets the Manifest state to `StateDeleting` with operation "waiting for module crs deletion" and returns the error. `delete()` calls `r.finishReconcile(...)` which persists the status and returns the error; controller-runtime requeues via exponential backoff.

**Two-phase delete model alignment ([discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442), [#833](https://github.com/kyma-project/lifecycle-manager/issues/833)):**  
Path B maps to Phase 1 ("await-for-default-cr-removal"): module resources remain active on the SKR so that the module operator can process the Default Module CR's deletion — `SyncResources(full_target)` re-applies them every reconcile. Path A maps to Phase 2 ("deprovision-resources"): once all CRs are gone, module resources are removed via `pruneDiff(empty_target)`.

Per the decision from [#833](https://github.com/kyma-project/lifecycle-manager/issues/833), KLM keeps module resources (RBAC, CRDs, module operator) up to date during Phase 1. If a user re-enables a module while deletion is in progress, KLM continues the in-progress deletion to completion before starting a fresh installation.

### 4.4 Listing across CRD versions (Huskies hotfix)
- Introduced by [PR #3021](https://github.com/kyma-project/lifecycle-manager/pull/3021) after [#3005](https://github.com/kyma-project/lifecycle-manager/issues/3005).
- [`listResourcesByGroupKindInAllNamespaces`](../../internal/manifest/modulecr/client.go#L178) enumerates every version registered by `RESTMapper` for the `GroupKind` and issues one cross-namespace `List` per version, concatenating results.
- Rationale: after an in-place API version upgrade the manifest reference points at the new version, but the SKR may still hold instances of the old version. The list must not miss them.

### 4.5 Cluster-scoped defensive path
- Introduced by [PR #3046](https://github.com/kyma-project/lifecycle-manager/pull/3046).
- [`isResourceTheDefaultCR`](../../internal/manifest/modulecr/client.go#L217-L236) treats namespaces as matching when either the found resource has empty `metadata.namespace` OR the RESTMapper reports the CRD as cluster-scoped (`meta.RESTScopeNameRoot`).
- This masks — for the deletion / filter-out path only — the namespace pollution introduced by `setNameAndNamespaceIfEmpty`.

### 4.6 Cluster-scoped status masking
- [`ModuleStatusGenerator.GenerateModuleStatus`](../../internal/service/kyma/status/modules/generator/generator.go#L83-L100) copies `Manifest.Spec.Resource.Namespace` into `Kyma.Status.Modules[].Resource.PartialMeta.Namespace`, then blanks it out if `ModuleTemplate` carries the annotation `operator.kyma-project.io/is-cluster-scoped` ([`shared.IsClusterScopedAnnotation`](../../api/shared/operator_annotations.go#L12)).

### 4.7 Callers (production only)

| Site | Method |
|---|---|
| [`labelsremoval/labels_removal.go:80`](../../internal/manifest/labelsremoval/labels_removal.go#L80) | `GetDefaultCR` |
| [`controller/manifest/controller.go:374`](../../internal/controller/manifest/controller.go#L374) | `RemoveDefaultModuleCR` |
| [`controller/manifest/controller.go:582`](../../internal/controller/manifest/controller.go#L582) | `CheckModuleCRsDeletion` |
| [`controller/manifest/controller.go:586`](../../internal/controller/manifest/controller.go#L586) | `CheckDefaultCRDeletion` |
| [`controller/manifest/controller.go:606`](../../internal/controller/manifest/controller.go#L606) | `SyncDefaultModuleCR` |

Every call site constructs `modulecr.NewClient(skrClient)` inline; there is no long-lived field and no interface abstraction. `cmd/main.go` does not import the package.

### 4.8 Related direct-access sites (bypassing the client)
- [`purge/controller.go:265-289`](../../internal/controller/purge/controller.go#L265) — `getAllRemainingCRs` lists CRs of any CRD and strips finalizers as part of the SKR-deprovision purge. Can affect Module CRs when a Kyma is being deprovisioned past the grace period.

---

## 5. Known Gaps and Open Topics

### 5.1 Correctness gaps

#### G1. Namespace pollution for cluster-scoped Module CRs
**Symptom.** `setNameAndNamespaceIfEmpty` at [template_to_module.go:141-144](../../internal/service/manifest/parser/template_to_module.go#L141-L144) unconditionally writes `remoteSyncNamespace` onto `ModuleTemplate.Spec.Data` if empty, regardless of whether the module's CRD is cluster-scoped. This value flows into `Manifest.Spec.Resource.Namespace`.

**Downstream impact.**
- `GetDefaultCR` at [client.go:56](../../internal/manifest/modulecr/client.go#L56) — uses the polluted namespace directly in `client.ObjectKey`. Unmitigated.
- `SyncDefaultModuleCR` at [client.go:130](../../internal/manifest/modulecr/client.go#L130) — passes the polluted resource straight to `Create`. Unmitigated. (Kubernetes may strip the namespace on a cluster-scoped CR; behavior needs empirical confirmation.)
- `isResourceTheDefaultCR` at [client.go:217](../../internal/manifest/modulecr/client.go#L217) — mitigated by the "defensive approach" (empty namespace OR RESTMapper scope=Root).
- `ModuleStatusGenerator` at [generator.go:83](../../internal/service/kyma/status/modules/generator/generator.go#L83) — mitigated by post-hoc masking based on `IsClusterScopedAnnotation`.

**Open question — cluster-scope signal.** Three signals exist and can disagree:
- The `operator.kyma-project.io/is-cluster-scoped` annotation on `ModuleTemplate` (used by the status generator only).
- The SKR's RESTMapper scope (used by the modulecr client only).
- The empty-namespace observation on a returned resource (used by the modulecr client only).

Deferred to team decision: which signal is authoritative, and whether to consolidate at the parser layer, the client layer, or both.

#### G2. `IsNotFound` broad aggregation and silent error swallows

`util.IsNotFound` ([pkg/util/error.go:32](../../pkg/util/error.go#L32)) aggregates five distinct conditions: `machineryruntime.IsNotRegisteredError`, `meta.IsNoMatchError` (CRD absent from cluster), `apierrors.IsNotFound` (HTTP 404), `discovery.ErrGroupDiscoveryFailed` containing a 404, and two string-based fallbacks ("failed to get restmapping", "could not find the requested resource"). The string fallbacks are the most fragile: a transient error whose message happens to contain those substrings would be classified as "not found" and silently treated as "no CRs exist."

Beyond the aggregation, `SyncDefaultModuleCR` ([client.go:126](../../internal/manifest/modulecr/client.go#L126)) has a distinct silent-swallow bug: the condition is `err != nil && util.IsNotFound(err)`. If `Get` returns a non-NotFound error (transient API failure, RBAC error, network timeout), the condition evaluates to false, the create block is skipped, and the function returns nil. The error is never surfaced in the manifest status; the `ModuleCR` condition is not set to True; the next reconcile retries silently. See also G11.

The four `util.IsNotFound` call sites on `listResourcesByGroupKindInAllNamespaces` results ([client.go:73](../../internal/manifest/modulecr/client.go#L73), [client.go:85](../../internal/manifest/modulecr/client.go#L85), [client.go:264](../../internal/manifest/modulecr/client.go#L264)) are semantically intentional — if the CRD is gone, there are no CRs. But the string fallbacks create ambiguity: a RESTMapper returning "failed to get restmapping" for a transient reason would be treated the same as a genuinely missing CRD. The intended semantics need to be specified per call site.

#### G3. Version-agnostic actions and installed-version tracking
The current strategy for cross-version safety is to iterate every version reported by the SKR RESTMapper. Alternatives raised by the ticket:
- Read the CRD from the SKR and pick the storage version or served versions explicitly.
- Persist the installed version in `Manifest.Status` and query that version directly.

Today `shared.Status` has no `InstalledVersion` field ([`api/shared/status.go:11-30`](../../api/shared/status.go#L11-L30)). Choice deferred.

#### G4. Dropping a CRD version safely
- Kubernetes forbids removing a version from `spec.versions` while it still appears in `status.storedVersions`. See [#2807](https://github.com/kyma-project/lifecycle-manager/issues/2807).
- KLM already implements [`DropStoredVersion`](../../internal/crd/storage_version_dropper.go) for KCP CRDs. No equivalent runs on SKRs today for Module CRDs.
- Per [Tomasz's argument in #2905](https://github.com/kyma-project/lifecycle-manager/issues/2905), KLM cannot know whether the module operator has migrated all CR instances on a given SKR. An explicit contract with module operators is required — a status flag or annotation set by the module operator to signal "safe to drop version X".

**Candidate contract designs (see also [discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442)):**
- **Option A — Per-instance annotation (primary contract):** The module operator stamps `operator.kyma-project.io/current-storage-version: "<new-version>"` on every CR instance after migration. KLM verifies all instances carry this annotation before calling `DropStoredVersion`. Requires active participation from the module operator.
- **Option B — CRD-level annotation (secondary contract):** The module operator adds `operator.kyma-project.io/dropping-storage-version: "<old-version>"` to the CRD itself as an explicit "migration complete, drop this version" signal. Lower verification granularity but simpler to implement.
- **Option C — No KLM-managed dropping:** Module teams handle version drops through their own means (migration jobs, custom controllers). KLM documents the constraint. This avoids adding cross-owner coupling to KLM.

Options A and B can be combined: B as the intent signal, A as the per-instance verification. The choice is deferred — see C2.

#### G5. Redundant list traversals and distributed CRP branching in the deletion gate
`CheckModuleCRsDeletion` and `CheckDefaultCRDeletion` both invoke `listResourcesByGroupKindInAllNamespaces` on the same GroupKind back-to-back, with no result sharing.

The CRP logic is split across two functions in ways that are hard to reason about without reading both:
- `CheckDefaultCRDeletion` short-circuits immediately for `CRP: Ignore` at [client.go:67](../../internal/manifest/modulecr/client.go#L67) (compound `Spec.Resource == nil || CRP == Ignore`).
- `GetAllModuleCRsExcludingDefaultCR` (called by `CheckModuleCRsDeletion`) includes ALL CRs — not filtering out the default — for `CRP: Ignore` at [client.go:157-162](../../internal/manifest/modulecr/client.go#L157-L162).

The two branches cancel each other out: the combined gate is R1-compliant (all CRs of the GroupKind must be gone, regardless of CRP). But the distributed branching is opaque and could introduce regressions in future edits. The two checks can be collapsed into a single "any CR of the GroupKind exists?" function that is explicitly CRP-independent.

#### G6. Renamed / moved Default Module CR
If a module team changes `ModuleTemplate.Spec.Data.metadata.name` or `metadata.namespace` between versions:
- The Manifest's new `Spec.Resource` no longer points at the previously created CR.
- `SyncDefaultModuleCR` would `Create` a new one alongside the old.
- `RemoveDefaultModuleCR` would target only the new one; the old one becomes a "customer CR" from KLM's perspective and blocks deletion until it is removed manually.

The ticket flags this as currently undefined. Options: reject the change via validation, support the rename explicitly (would require some form of previous-name tracking), or document it as a known limitation for module teams.

### 5.2 Architectural / ADR gaps

#### G7. No consumer-defined interface (ADR 001)
Callers depend on the concrete `*modulecr.Client` struct. No interface exists at the consumer side, and no mock is generated. Test coverage relies on the real client against a fake `client.Client`.

#### G8. Inline construction violates composition-root DI (ADR 002)
`NewClient` is called on the fly in five production sites. Each call receives an already-resolved SKR client, but the modulecr client itself is not composed at `cmd/main.go`. A single service, constructed once, would let callers receive it via constructor injection.

#### G9. Layering (ADR 004)
The package sits at `internal/manifest/modulecr/`, i.e., not under `service/` nor `repository/`. Its methods mix service-level orchestration (`SyncDefaultModuleCR`, `RemoveDefaultModuleCR`) with repository-level access (`listResourcesByGroupKindInAllNamespaces`, `GetDefaultCR`). Splitting into a `ModuleCRService` and a `ModuleCRRepository` would align with the layered architecture.

#### G10. Naming and dead surface (ADR 005 and general cleanup)
- `SyncDefaultModuleCR` is misleading — it is create-if-missing, never sync. Consider `EnsureDefaultCRCreated`.
- `GetAllModuleCRsExcludingDefaultCR` is public but has no callers outside the `internal/manifest/modulecr/` package; it is called only from `CheckModuleCRsDeletion` internally and directly from tests. Unexport it or move the tests to use `CheckModuleCRsDeletion` instead.
- `Client` in a package named `modulecr` violates ADR 005's "let the package provide the context" — should be `ModuleCRService`/`ModuleCRRepository` per ADR 005.

#### G11. `LabelRemovalFinalizer` permanently stuck if default CR is manually deleted before unmange
`removeFromDefaultCR` at [labels_removal.go:72-86](../../internal/manifest/labelsremoval/labels_removal.go#L72-L86) has no NotFound tolerance. If the default Module CR is manually deleted before the module is unmanaged, `GetDefaultCR` returns a wrapped NotFound error. `RemoveManagedByLabel` receives that error and returns it without removing the `LabelRemovalFinalizer` or calling `UpdateManifest`. On subsequent reconciles, the same flow repeats. The `LabelRemovalFinalizer` is never removed and the Manifest is stuck.

#### G12. `SyncDefaultModuleCR` silently swallows non-NotFound errors from `Get`
Exact condition at [client.go:126](../../internal/manifest/modulecr/client.go#L126):
```go
if err := c.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil && util.IsNotFound(err) {
```
When `Get` returns an error that is NOT NotFound-class (transient API failure, network timeout, RBAC denial), the condition evaluates to false. The create block is skipped and the function returns nil. The error is never stored or surfaced. The `ModuleCR` condition remains False, so the next reconcile retries — but the caller never enters StateError. Persistent failures (for example, permanent RBAC) loop silently indefinitely.

#### G13. Shared Module CRD between two modules
`listResourcesByGroupKindInAllNamespaces` queries by `GroupKind` only, with no filtering by owner, Manifest, or label. If two different modules ship the same `GroupKind` — for example, a shared API group or a module split across two Manifests — both Manifests' deletion gates block on each other's CRs. There is no partitioning between modules for this query; which Manifest unblocks first depends on the order in which CRs are deleted, making the outcome non-deterministic. No error or warning is produced. [community#982](https://github.com/kyma-project/community/issues/982) assumes each Default CR belongs exclusively to one module, but the current code cannot enforce this.

### 5.3 UX / product open topics

- **UX evolution ([#2428](https://github.com/kyma-project/lifecycle-manager/issues/2428)).** community#972 addendum flags an alternative flow where KLM never installs the Default Module CR and the UI/CLI prompts the user to configure the module. Would change or deprecate `CRP: CreateAndDelete`. Currently exploratory.
- **Second-tier customer CR blocking.** community#972 addendum notes that with customer-created CRs present, KLM only deletes the Default CR once the customer CRs are gone. This is by design under R3 but is user-visible; may warrant clearer UI messaging.
- **Two-phase delete formalization ([discussion #3442](https://github.com/kyma-project/lifecycle-manager/discussions/3442)).** The current Path A/B/C behavior informally implements a two-phase model. A formal `await-for-cr-removal` → `deprovision-resources` state machine on the Manifest, with explicit state transitions and observability, would make the behavior debuggable and extensible. No accepted decision yet.

---

## 6. Explicit Non-Goals

- **Market-specific Module CRs.** Different Default Module CRs per market (originally requested by NATS) are not supported and there is no code path for it.
- **Reporting module configuration status.** KLM does not surface configuration-level state of a module in `Kyma.Status`. Configuration validity is the concern of the module operator and, where applicable, the UI/CLI.
- **Managing customer-created CRs.** KLM never creates, mutates, or deletes customer-created Module CRs. It only observes them as part of the deletion gate.
- **Migrating existing CR instances between CRD versions.** This is the module operator's responsibility. KLM neither performs conversion nor validates that migration has taken place before applying CRD version changes (see G4).

---

## 7. Follow-Up Work

The ticket calls for follow-ups covering the gaps above. Draft proposal below; to be estimated and split in backlog refinement.

### EPIC: Robust Module CR handling
_Umbrella tracking [#3029](https://github.com/kyma-project/lifecycle-manager/issues/3029) outcomes. Groups the correctness, refactor, and contract work._

#### Track A — Correctness

- **A1.** Stop defaulting namespace for cluster-scoped Module CRs at the parser layer. Depends on the cluster-scope-signal decision (G1, deferred).
- **A2.** Consolidate `CheckDefaultCRDeletion` + `CheckModuleCRsDeletion` into a single CRP-independent "any CR of the GroupKind exists?" gate, eliminating the duplicate list traversal and the distributed CRP branching. (G5)
- **A3.** Define expected error classes per call site for `util.IsNotFound` usage; distinguish "CRD absent" (intentional no-op) from transient API-server errors (surface as error). (G2)
- **A4.** Add `Manifest.Status.InstalledVersion` (or equivalent) and query that version directly instead of iterating RESTMappings. (G3) _Depends on team decision between status field vs. reading CRD storage version._
- **A5.** Empirically verify `SyncDefaultModuleCR` for cluster-scoped CRs; fix if the create currently fails or misbehaves with a namespaced spec. (G1)
- **A6.** Fix `removeFromDefaultCR` to tolerate NotFound on `GetDefaultCR` — if the default CR is already gone, the label-removal step should succeed rather than leaving the `LabelRemovalFinalizer` stuck. (G11)
- **A7.** Fix `SyncDefaultModuleCR` to surface non-NotFound `Get` errors: change the condition so that any error from `Get` that is NOT NotFound is returned rather than silently discarded. (G12)

#### Track B — Refactor (ADR alignment)

- **B1.** Extract a consumer-defined interface (`ModuleCRService` or narrower) at each caller; make callers depend on interfaces. (G7, ADR 001)
- **B2.** Split into `internal/service/manifest/modulecr` (orchestration) and `internal/repository/manifest/modulecr` (K8s I/O). (G9, ADR 004)
- **B3.** Wire the service once at composition root; remove the five inline `NewClient` sites. (G8, ADR 002)
- **B4.** Rename `SyncDefaultModuleCR` to `EnsureDefaultCRCreated`; unexport or remove `GetAllModuleCRsExcludingDefaultCR`. (G10, ADR 005)

#### Track C — Contract and open questions (require team decision)

- **C1.** Decide the authoritative cluster-scope signal (annotation, RESTMapper, both). (G1)
- **C2.** Define the "safe to drop version" contract with module operators. Spec proposal, then implementation. (G4, [#2807](https://github.com/kyma-project/lifecycle-manager/issues/2807), [#2905](https://github.com/kyma-project/lifecycle-manager/issues/2905))
- **C3.** Define behavior for renamed/moved Default Module CR — prevent via validation, support explicitly, or document as known gap. (G6)
- **C4.** Follow up on [#2428](https://github.com/kyma-project/lifecycle-manager/issues/2428) — evolution of `CRP: CreateAndDelete` with UI/CLI-driven configuration.
- **C5.** Define and implement the formal two-phase delete state machine (`await-for-cr-removal` → `deprovision-resources`) as explicit Manifest states with observable transitions. (discussion #3442, [#833](https://github.com/kyma-project/lifecycle-manager/issues/833))

---

## 8. References

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
