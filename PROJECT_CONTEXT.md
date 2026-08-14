# Kyma Lifecycle Manager (KLM) - Comprehensive Project Context

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Architecture](#2-architecture)
3. [Custom Resource Definitions (CRDs)](#3-custom-resource-definitions-crds)
4. [Controllers](#4-controllers)
5. [Service Layer](#5-service-layer)
6. [Repository Layer](#6-repository-layer)
7. [Remote Cluster Management](#7-remote-cluster-management)
8. [Declarative Reconciliation Engine](#8-declarative-reconciliation-engine)
9. [Template Lookup & Module Resolution](#9-template-lookup--module-resolution)
10. [Maintenance Windows](#10-maintenance-windows)
11. [Certificate Management & PKI](#11-certificate-management--pki)
12. [Labels, Annotations & Constants](#12-labels-annotations--constants)
13. [Configuration Flags](#13-configuration-flags)
14. [Metrics & Observability](#14-metrics--observability)
15. [Testing Infrastructure](#15-testing-infrastructure)
16. [CI/CD Pipelines](#16-cicd-pipelines)
17. [Build & Deployment](#17-build--deployment)
18. [Architecture Decision Records (ADRs)](#18-architecture-decision-records-adrs)
19. [Key Design Patterns](#19-key-design-patterns)
20. [Directory Structure](#20-directory-structure)
21. [Dependencies](#21-dependencies)
22. [Tool Versions](#22-tool-versions)
23. [Incident Debugging — Cluster Access](#23-incident-debugging--cluster-access)

---

## 1. Project Overview

**Kyma Lifecycle Manager (KLM)** is a Kubernetes meta-operator that coordinates and tracks the lifecycle of Kyma modules across distributed clusters. It is a core component of **SAP BTP, Kyma runtime** — an opinionated set of Kubernetes-based modular building blocks for developing and running cloud-native applications.

### Core Mission

KLM operates within the **KCP (Kyma Control Plane)** cluster and manages the lifecycle of Kyma modules on **SKR (SAP BTP, Kyma Runtime)** clusters — hyperscaler-provisioned clusters for SAP BTP customers.

### Key Responsibilities

1. Installing CRDs required for module deployment on SKR clusters
2. Synchronizing the module catalog (ModuleTemplates + ModuleReleaseMetas) to SKR clusters
3. Installing, updating, reconciling, and deleting module resources on SKR clusters
4. Watching SKR clusters for user-requested changes and propagating them back to KCP
5. Managing mandatory modules across all clusters
6. Enforcing maintenance windows for disruptive module updates
7. Handling certificate rotation with zero downtime

### Multi-Cluster Model

```
┌─────────────────────────────────────────────────┐
│                 KCP CLUSTER                       │
│  ┌─────────────────────────────────────────────┐ │
│  │ Kyma Lifecycle Manager (kcp-system)          │ │
│  │  - Kyma Controller                          │ │
│  │  - Manifest Controller                      │ │
│  │  - Mandatory Module Controllers             │ │
│  │  - Purge Controller                         │ │
│  │  - Watcher Controller                       │ │
│  │  - Istio Gateway Secret Controller          │ │
│  └─────────────────────────────────────────────┘ │
│  ┌─────────────────────────────────────────────┐ │
│  │ Resources (kcp-system)                       │ │
│  │  - Kyma CRs (one per SKR cluster)           │ │
│  │  - Manifest CRs (one per module per SKR)    │ │
│  │  - ModuleTemplate CRs (module definitions)  │ │
│  │  - ModuleReleaseMeta CRs (channel→version)  │ │
│  │  - Watcher CRs (webhook routing)            │ │
│  │  - Access Secrets (kubeconfigs per SKR)      │ │
│  └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
         │                    ▲
         │ SSA Deploy         │ Events via
         │ Sync Catalog       │ Istio Gateway
         ▼                    │
┌─────────────────────────────────────────────────┐
│                SKR CLUSTER(S)                     │
│  ┌─────────────────────────────────────────────┐ │
│  │ kyma-system namespace                        │ │
│  │  - Kyma CR (synced from KCP)                 │ │
│  │  - ModuleTemplate CRs (catalog)             │ │
│  │  - ModuleReleaseMeta CRs (catalog)          │ │
│  │  - Module CRs (deployed modules)            │ │
│  │  - SKR Webhook (ValidatingWebhook)          │ │
│  └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

---

## 2. Architecture

### 3-Tier Layered Architecture (ADR 004)

```
┌────────────────────────────────────────────────────────┐
│  CONTROLLER LAYER (internal/controller/)                │
│  Orchestrates reconciliation, handles event processing  │
│  and requeue decisions. Entry point for controller-     │
│  runtime reconcile loops.                               │
├────────────────────────────────────────────────────────┤
│  SERVICE LAYER (internal/service/)                       │
│  Implements business logic and workflows. Orchestrates   │
│  repository calls. Contains use cases, validation,      │
│  and complex multi-step operations.                     │
├────────────────────────────────────────────────────────┤
│  REPOSITORY LAYER (internal/repository/)                │
│  Pure CRUD operations. Directly interacts with the      │
│  Kubernetes API. No business logic.                     │
└────────────────────────────────────────────────────────┘
```

**Rules:**
- Dependencies flow ONLY downward
- No layer may reference a higher layer
- `client.Client` is used ONLY in the Repository layer (ADR 003)
- Specific sub-interfaces (Reader, Writer) preferred over full Client
- Interfaces defined at the consumer side (ADR 001)
- Dependencies injected via constructors (`New<StructName>`) (ADR 002)

### Module Deployment Workflow

1. Each module consists of a **manager** (Deployment/StatefulSet) and a **custom resource**
2. Admin adds/removes modules via Kyma CR's `spec.modules[]`
3. KLM reads ModuleTemplate and ModuleReleaseMeta CRs to resolve module definitions
4. KLM creates/updates Manifest CRs representing resources to install per module
5. Manifest controller deploys module resources to SKR cluster via Server-Side Apply
6. Module manager (operator) reconciles the module CR on SKR
7. State is aggregated into Kyma CR status

### Synchronization Model

- **Spec direction:** SKR → KCP (SKR is single source of truth for desired state)
- **Status direction:** KCP → SKR (KLM writes aggregated status back to SKR)
- **Catalog direction:** KCP → SKR (filtered by visibility: default/internal/beta)

---

## 3. Custom Resource Definitions (CRDs)

All CRDs are in API group `operator.kyma-project.io`, storage version `v1beta2` (v1beta1 deprecated with conversion).

### 3.1 Kyma CRD

**Scope:** Namespaced | **Short Name:** none

The root configuration resource representing a Kyma installation instance.

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Kyma
metadata:
  name: default
  namespace: kcp-system
  labels:
    operator.kyma-project.io/managed-by: kyma
    kyma-project.io/runtime-id: <runtime-id>
    kyma-project.io/global-account-id: <account-id>
spec:
  channel: regular                    # Global channel (pattern: ^[a-z]+$, 3-32 chars)
  skipMaintenanceWindows: false       # Override maintenance windows
  modules:                            # List of desired modules
    - name: keda                      # Module identifier
      channel: fast                   # Per-module channel override (optional)
      customResourcePolicy: CreateAndDelete  # or Ignore
      managed: true                   # Whether operator manages the module
status:
  state: Ready                        # Aggregated state
  activeChannel: regular
  conditions:
    - type: Modules
      status: "True"
      reason: Ready
      message: "all modules are in ready state"
    - type: ModuleCatalog
      status: "True"
    - type: SKRWebhook
      status: "True"
  modules:                            # Per-module status
    - name: keda
      state: Ready
      channel: fast
      version: 1.2.3
      manifest: {name: keda-manifest, namespace: kcp-system, generation: 5}
      template: {name: keda-1.2.3, namespace: kcp-system, generation: 2}
      resource: {name: keda, namespace: kyma-system}
```

**Condition Types:**
- `Modules` — All modules ready/not ready
- `ModuleCatalog` — Module templates synchronized
- `SKRWebhook` — SKR webhook synchronized
- `SKRImagePullSecretSync` — Image pull secrets synchronized

**State Machine:** Empty → Processing → Ready/Warning/Error → Deleting

**Finalizers:** `operator.kyma-project.io/kyma`, `operator.kyma-project.io/purge-finalizer`

### 3.2 Manifest CRD

**Scope:** Namespaced | **Short Name:** none

Represents a single module's deployment state. Created dynamically by Kyma controller.

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Manifest
metadata:
  name: keda-manifest
  namespace: kcp-system
  labels:
    operator.kyma-project.io/kyma-name: default
    operator.kyma-project.io/module-name: keda
    operator.kyma-project.io/channel: fast
spec:
  remote: true                        # Deploy to remote SKR cluster
  version: "1.2.3"
  customResourcePolicy: CreateAndDelete
  install:
    source: {type: oci-ref, repo: ..., name: ..., ref: ...}
    name: keda
  resource: {}                        # Module CR to watch for state
  manager:                            # Module manager (Deployment/StatefulSet)
    group: apps
    version: v1
    kind: Deployment
    name: keda-manager
    namespace: kyma-system
  localizedImages: []                 # Docker image references for rewriting
status:
  state: Ready
  conditions:
    - type: Resources
      status: "True"
      reason: ResourcesAvailable
    - type: Installation
      status: "True"
      reason: Ready
```

**Layers:**
- `raw-manifest` — Main module resources (Helm chart/kustomize/raw)
- `default-cr` — Default Custom Resource for the module
- `config` — Configuration overlay

### 3.3 ModuleTemplate CRD

**Scope:** Namespaced | **Short Name:** `mt`

Defines a module at a specific version with OCM descriptor and default configuration.

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleTemplate
metadata:
  name: keda-1.2.3
  namespace: kcp-system
  labels:
    operator.kyma-project.io/managed-by: kyma
spec:
  moduleName: keda                    # Pattern: ^([a-z]{3,}(-[a-z]{3,})*)?$
  version: "1.2.3"                    # Semver pattern
  mandatory: false
  requiresDowntime: false             # Triggers maintenance window enforcement
  descriptor: <RawExtension>          # OCM Component Descriptor
  data: <Unstructured>                # Default CR attributes (new modules only)
  resources:                          # Additional resources
    - name: raw-manifest
      link: "oci://registry/path:tag"
  manager:
    group: apps
    version: v1
    kind: Deployment
    name: keda-manager
    namespace: kyma-system
  info:
    repository: "https://github.com/..."
    documentation: "https://..."
  associatedResources:                # For cleanup on removal
    - group: operator.kyma-project.io
      version: v1alpha1
      kind: Keda
```

### 3.4 ModuleReleaseMeta CRD

**Scope:** Namespaced | **Short Name:** `mrm`

Maps module versions to release channels. Exactly one of `channels` or `mandatory` must be specified.

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleReleaseMeta
metadata:
  name: keda
  namespace: kcp-system
spec:
  moduleName: keda
  ocmComponentName: "kyma-project.io/module/keda"
  channels:                           # Mutually exclusive with mandatory
    - channel: regular
      version: "1.2.3"
    - channel: fast
      version: "1.3.0"
  # OR
  mandatory:                          # Mutually exclusive with channels
    version: "2.0.0"
  beta: false                         # Deprecated
  internal: false                     # Deprecated
  kymaSelector:                       # EXPERIMENTAL: restrict to matching Kymas
    matchLabels: {}
    matchExpressions: []
```

### 3.5 Watcher CRD

**Scope:** Namespaced | **Short Name:** none

Configures webhook-based monitoring of SKR resources for change propagation back to KCP.

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Watcher
metadata:
  name: klm-watcher
  namespace: kcp-system
spec:
  resourceToWatch:
    group: operator.kyma-project.io
    version: v1beta2
    resource: kymas
  field: spec                         # spec or status
  serviceInfo:
    name: klm-controller-manager-events
    namespace: kcp-system
    port: 8082
  gateway:
    selector:
      matchLabels:
        operator.kyma-project.io/purpose: klm-watcher-cert-manager
status:
  state: Ready
  conditions:
    - type: VirtualService
      status: "True"
      reason: Ready
      message: "VirtualService is configured"
```

---

## 4. Controllers

### 4.1 Kyma Controller

**Location:** `internal/controller/kyma/controller.go`
**Setup:** `internal/controller/kyma/setup.go`

The main orchestration controller managing the Kyma CR lifecycle.

**Reconciliation Flow:**
1. Fetch Kyma CR (handle deleted case)
2. Initialize conditions
3. Check skip-reconciliation label
4. Initialize SKR context (get remote client from access secret)
5. Handle deletion (11-step deletion workflow via DeletionService)
6. Create SKR namespace (`kyma-system`)
7. Ensure labels and finalizers
8. Sync CRDs to SKR
9. Sync image pull secrets (if enabled)
10. Replace local spec with remote spec (SKR is source of truth)
11. Process state machine:
    - **Processing:** Reconcile manifests → sync catalog → sync webhook → update statuses → determine state
    - **Ready/Warning:** Continue monitoring
    - **Error:** Retry with rate limiting
12. Sync status back to SKR

**Event Sources:**
- Kyma CR generation/label changes
- ModuleTemplate changes (requeues related Kymas)
- ModuleReleaseMeta changes (requeues related Kymas)
- Secret changes (access credentials)
- Manifest ownership changes
- SKR events via runtime-watcher listener (port 8082)

### 4.2 Manifest Controller

**Location:** `internal/declarative/v2/reconciler.go`
**Setup:** `internal/controller/manifest/setup.go`

Manages Manifest CRs — deploys module resources to SKR via Server-Side Apply.

**Reconciliation Flow:**
1. Fetch Manifest CR
2. Check skip-reconciliation label
3. Initialize status conditions
4. Resolve SKR client (with caching, 23-25h TTL)
5. Handle unmanaged manifests
6. Detect orphaned manifests (5-minute tolerance for new ones)
7. Add finalizers
8. Resolve spec (parse OCI template)
9. Track synced OCI ref annotation
10. Render resources (parse + apply transforms)
11. Prune obsolete resources (diff against previous sync)
12. Sync resources to SKR via SSA
13. Check state (Deployment/StatefulSet readiness)
14. Update manifest state
15. Handle deletion (cleanup remote resources)

**Resource Transforms (applied in order):**
1. `ManagedByOwnedBy` — Adds managed-by label and owned-by annotation
2. `KymaComponentTransform` — Adds component and part-of labels
3. `DisclaimerTransform` — Adds "DO NOT EDIT" disclaimer annotation
4. `DockerImageLocalizationTransform` — Rewrites container images
5. `SkrImagePullSecretTransform` — Injects image pull secrets (optional)

### 4.3 Mandatory Module Installation Controller

**Location:** `internal/controller/mandatorymodule/installation_controller.go`

Installs mandatory modules on all Kyma instances.

**Flow:**
1. Validate Kyma state (not deleting, not skipped)
2. List all ModuleReleaseMeta marked as mandatory
3. For each mandatory MRM, fetch ModuleTemplate by version
4. Generate Module objects from templates
5. Reconcile Manifests (create/update)
6. Record metrics (mandatory module count)

### 4.4 Mandatory Module Deletion Controller

**Location:** `internal/controller/mandatorymodule/deletion_controller.go`

Handles deletion of mandatory module ModuleReleaseMetas.

**Ordered Steps:**
1. EnsureFinalizer — Add `mandatory-module` finalizer
2. SkipNonDeleting — Skip if MRM not being deleted
3. DeleteManifests — Remove all Manifests for the module
4. RemoveFinalizer — Remove finalizer to allow GC

### 4.5 Purge Controller

**Location:** `internal/controller/purge/controller.go`

Removes finalizers from stale SKR resources after a timeout to force cleanup.

**Flow:**
1. If Kyma not deleting: ensure purge-finalizer present
2. Check if purge deadline reached (DeletionTimestamp + PurgeFinalizerTimeout)
3. If deadline not reached: requeue
4. Get SKR client (handle case where SKR already deleted)
5. List all CRDs in SKR
6. For each CRD (except Kyma and skip list): remove finalizers from all CRs
7. Remove purge-finalizer from Kyma CR
8. Record purge duration metric

### 4.6 Watcher Controller

**Location:** `internal/controller/watcher/controller.go`

Manages Istio VirtualService rules for webhook-based change propagation from SKR.

**Flow:**
1. Fetch Watcher CR
2. Add watcher finalizer
3. Initialize conditions
4. **Processing:** List Gateways → Create/Update VirtualService → Set Ready
5. **Deleting:** Delete VirtualService → Remove finalizer
6. Update condition status

### 4.7 Istio Gateway Secret Controller

**Location:** `internal/controller/istiogatewaysecret/controller.go`

Manages certificate rotation for Istio gateway with zero downtime.

**Flow:**
1. Get root gateway secret (`klm-watcher` in `istio-system`)
2. Delegate to GatewaySecretHandler for certificate bundling
3. Manage rotation window (grace period: 4 days, expiry window: 14 days)
4. Requeue on configured intervals

---

## 5. Service Layer

### 5.1 Kyma Deletion Service

**Location:** `internal/service/kyma/deletion/`

Implements a strictly ordered 11-step workflow for Kyma instance deletion:

| Step | Use Case | Description |
|------|----------|-------------|
| 1 | SetKcpKymaStateDeleting | Set KCP Kyma status to "Deleting" |
| 2 | SetSkrKymaStateDeleting | Set SKR Kyma status to "Deleting" |
| 3 | DeleteSkrKyma | Delete Kyma resource from SKR |
| 4 | DeleteWatcherCertificateSetup | Remove SSL certs and secrets |
| 5 | DeleteSkrWebhookResources | Remove webhook config from SKR |
| 6 | DeleteSkrModuleTemplateCrd | Delete ModuleTemplate CRD from SKR |
| 7 | DeleteSkrModuleReleaseMetaCrd | Delete ModuleReleaseMeta CRD from SKR |
| 8 | DeleteSkrKymaCrd | Delete Kyma CRD from SKR |
| 9 | DeleteManifests | Remove all Manifest CRs for the Kyma |
| 10 | DeleteMetrics | Clean up Prometheus metrics |
| 11 | DropKymaFinalizer | Remove finalizer (allows GC) |

Each step implements `UseCase` interface with `IsApplicable()` and `Execute()` methods. Order is validated at construction time.

### 5.2 Kyma Status Service

**Location:** `internal/service/kyma/status/`

- **StatusHandler:** Updates module statuses and cleans up deleted modules
- **ModuleStatusGenerator:** Generates status from module state, template, manifest tracking
- **Error Status Generator:** Maps errors to module states:
  - `ErrWaitingForNextMaintenanceWindow` → current state + maintenance=true
  - `ErrTemplateUpdateNotAllowed` → Warning
  - `ErrNoModuleReleaseMeta` → Warning
  - Default errors → Error

### 5.3 Mandatory Module Service

**Location:** `internal/service/mandatorymodule/`

- **Installation Service:** Lists mandatory MRMs → fetches templates by version → generates modules → reconciles manifests
- **Deletion Service:** 4-step ordered workflow (EnsureFinalizer → SkipNonDeleting → DeleteManifests → RemoveFinalizer)

### 5.4 Restricted Module Service

**Location:** `internal/service/restrictedmodule/`

Determines if a ModuleReleaseMeta applies to a specific Kyma based on `kymaSelector` label matching.

### 5.5 SKR Client Service

**Location:** `internal/service/skrclient/`

Multi-layered Kubernetes client for SKR clusters:
- **Service:** Creates SKR clients from access secrets
- **SKRClient:** Wraps controller-runtime client with REST client caching
- **ProxyClient:** Validates REST mapper before each operation
- **KubeFactoryProxy:** Thread-safe cache for structured/unstructured REST clients
- **Client Cache:** TTL-based cache (23-25h random TTL to prevent stampede)

### 5.6 SKR Sync Service

**Location:** `internal/service/skrsync/`

- **SyncCrds:** Syncs CRD definitions from KCP to SKR
- **SyncImagePullSecret:** Copies image pull secrets to SKR (clears cluster-specific metadata)

### 5.7 Component Descriptor Service

**Location:** `internal/service/componentdescriptor/`

Retrieves OCM component descriptors from OCI registries:
1. Get descriptor layer digest from OCI artifact config
2. Pull layer from OCI repository
3. Extract component descriptor file from layer TAR (max 100KB, DoS protection)
4. Deserialize JSON into descriptor object

### 5.8 Access Manager Service

**Location:** `internal/service/accessmanager/`

Manages access to SKR clusters via kubeconfig secrets:
- Lists secrets by `operator.kyma-project.io/kyma-name` label
- Extracts kubeconfig and creates REST config
- Validates exactly one secret per Kyma

---

## 6. Repository Layer

**Location:** `internal/repository/`

| Repository | Resource | Key Methods |
|-----------|----------|-------------|
| `kyma/` | Kyma | Get, GetAll, LookupByLabel, DropFinalizer |
| `manifest/` | Manifest | DeleteAllForModule, ListAllForModule, ExistForKyma |
| `moduletemplate/` | ModuleTemplate | Get, GetByVersion |
| `modulereleasemeta/` | ModuleReleaseMeta | Get, List |
| `secret/` | Secret | Get, List |
| `ocm/` | OCI Registry | Pull, GetConfig |
| `istiogateway/` | Istio Gateway | ListBySelector |
| `watcher/certificate/` | Certificate | Create, Delete, Get |
| `skr/crd/` | SKR CRDs | List, Patch, Get |
| `skr/kyma/` | SKR Kyma | Get, Create, Patch, Delete |
| `skr/webhook/` | SKR Webhook | Manage webhook resources |

---

## 7. Remote Cluster Management

**Location:** `internal/remote/`

### SKR Context Provider

Factory for creating and caching SKR cluster connections:
- Uses `AccessManagerService` to get kubeconfig from secrets
- Caches clients with 23-25h random TTL (prevents thundering herd)
- Sets QPS/Burst from flags (default: 50/100 for SKR)
- Provides cache invalidation for connection errors

### CRD Synchronization

- Tracks CRD generations in Kyma annotations
- Compares KCP vs SKR CRD versions
- Patches SKR CRDs via SSA when updates detected
- Creates CRDs in SKR if missing
- Validates CRD readiness (Established + NamesAccepted conditions)

### Remote Catalog Sync

Synchronizes module catalog from KCP to SKR:

**Visibility Rules:**
- Default modules: synchronized to all clusters
- Internal modules: only if Kyma has `operator.kyma-project.io/internal: "true"` label
- Beta modules: only if Kyma has `operator.kyma-project.io/beta: "true"` label
- Mandatory modules: excluded from catalog sync (handled by dedicated controller)

**Sync Workers:**
- Concurrent goroutines for apply/delete operations
- CRD creation retry on NoMatchError
- Diff calculation: apply desired → list existing → delete obsolete
- Only deletes resources managed by `catalog-sync` field manager

---

## 8. Declarative Reconciliation Engine

**Location:** `internal/declarative/v2/`

The core engine for deploying module resources to SKR via Server-Side Apply (ADR 008).

### Why SSA?

- No preceding read necessary (reduces API calls across fleet)
- No diff computation needed (server handles conflicts)
- Upsert semantics (create or update transparently)
- Proper field ownership management
- Automatic removal of previously managed fields
- Scales to thousands of Kyma runtime instances

### Cached Manifest Parser

- TTL-based in-memory cache for parsed manifest resources
- Cache key: `manifest/<path>/<manifestName>`
- Returns deep copies to prevent mutation
- Evicts on OCI ref changes

### Resource Diff & Pruning

- Tracks synced resources in Manifest status
- Compares current resources vs previously synced
- Prunes (deletes) resources that are no longer in the manifest
- Prevents deletion if OCI ref hasn't changed (guards against cache issues)

### State Checking

- **ManagerStateCheck:** Checks Deployment/StatefulSet readiness
- **ExistsStateCheck:** Verifies all resources exist in cluster
- Custom state checkers configurable per module

---

## 9. Template Lookup & Module Resolution

**Location:** `pkg/templatelookup/`

### Resolution Flow

```
Kyma.spec.modules[].name
    → FetchModuleInfo() (validate config, collect enabled + previously installed)
    → GetModuleReleaseMeta() (find MRM by module name)
    → Determine channel (module channel > global channel > "regular")
    → Resolve version (MRM.channels[channel] or MRM.mandatory.version)
    → Fetch ModuleTemplate by name: "<moduleName>-<version>"
    → Validate visibility (internal/beta restrictions)
    → Check version skew (prevent downgrades)
    → MaintenanceWindowDecorator (gate if requiresDowntime)
    → Return ModuleTemplateInfo with resolved template + OCM identity
```

### Validation Rules

- Channel "none" is not allowed in Kyma spec
- Version and channel are mutually exclusive per module
- Downgrades prevented (semver comparison ignoring pre-release)
- Internal modules require `internal: "true"` label on Kyma
- Beta modules require `beta: "true"` label on Kyma

### Module Sync Runner

**Location:** `pkg/module/sync/runner.go`

Orchestrates Manifest CR updates:
- Concurrent processing of modules via goroutines
- `NeedToUpdate()` logic:
  - New manifest: always update (unless unmanaged)
  - Unmanaged manifest: never update
  - Module becoming unmanaged: always update (mark for deletion)
  - Mandatory module: update on spec diff
  - Regular: update on template generation diff or spec diff
- SSA patches with proper field ownership
- Owner references link Manifests to Kyma CR

---

## 10. Maintenance Windows

**Location:** `internal/maintenancewindows/`

### Policy-Based Architecture

- Policies loaded from JSON files at `/etc/maintenance-policy/`
- Policy resolution uses Runtime metadata: GlobalAccountID, Region, PlatformRegion, Plan
- Minimum window size configurable (default: 20 minutes)

### Enforcement Logic

```
IsRequired(template, kyma, currentStatus):
  if template.Spec.RequiresDowntime == false → false
  if kyma.Spec.SkipMaintenanceWindows == true → false
  if module not yet installed (no status) → false
  if version won't change → false
  return true

IsActive(kyma):
  resolve maintenance window from policy
  return now >= window.Begin && now <= window.End
```

### Integration via Decorator Pattern

`MaintenanceWindowDecorator` wraps the base `ModuleLookup` strategy:
1. Call wrapped lookup strategy
2. If maintenance required: check if window active
3. If not active: return `ErrWaitingForNextMaintenanceWindow`
4. Status generator maps this to: current state + `maintenance: true` flag

---

## 11. Certificate Management & PKI

### Architecture (ADR 007)

- Uses CA certificates only (no intermediate certificates)
- CA certificate serves as both CA and server certificate
- Supports two backends: cert-manager.io/v1 and cert.gardener.cloud/v1alpha1

### Zero-Downtime Rotation (6 Steps)

1. Rotate CA certificate (new cert issued)
2. Bundle old + new CA certificates in gateway secret
3. Trigger client certificate reissuance
4. Reissue client certificates on all SKR clusters
5. Sync new certificates to SKR
6. Switch server certificate after grace period (4 days)

### Gateway Secret Controller

- Watches `klm-watcher` secret in `istio-system`
- Grace period: 96 hours (4 days) after CA rotation
- Expiry window: 336 hours (14 days) before cert expiry
- Bundles old + new CA certificates during rotation

---

## 12. Labels, Annotations & Constants

### Labels (operator.kyma-project.io/ prefix)

| Label | Purpose |
|-------|---------|
| `controller-name` | Identifies the controller |
| `channel` | Module channel identifier |
| `managed-by` | Managing controller (value: "kyma") |
| `kyma-name` | Owning Kyma CR name |
| `module-name` | Module identifier |
| `mandatory-module` | Marks mandatory modules |
| `skip-reconciliation` | Skip reconciliation flag |
| `internal` | Internal resource flag |
| `beta` | Beta resource flag |
| `watched-by` | Watches redirect controller |
| `purpose` | Resource purpose identifier |

### Labels (kyma-project.io/ prefix)

| Label | Purpose |
|-------|---------|
| `global-account-id` | SAP BTP Global Account |
| `subaccount-id` | SAP BTP Subaccount |
| `region` | Cluster region |
| `platform-region` | Platform region |
| `broker-plan-name` | BTP plan identifier |
| `runtime-id` | Runtime instance ID |
| `instance-id` | Instance identifier |

### Annotations

| Annotation | Purpose |
|-----------|---------|
| `operator.kyma-project.io/fqdn` | Module FQDN (deprecated) |
| `operator.kyma-project.io/ocm-component-name` | OCM component name |
| `operator.kyma-project.io/is-cluster-scoped` | Cluster-scoped resource flag |
| `operator.kyma-project.io/is-unmanaged` | Unmanaged resource flag |
| `skr-domain` | SKR cluster FQDN |
| `operator.kyma-project.io/managed-by-reconciler-disclaimer` | DO NOT EDIT warning |

### Finalizers

| Finalizer | Purpose |
|-----------|---------|
| `operator.kyma-project.io/kyma` | Kyma resource lifecycle |
| `operator.kyma-project.io/purge-finalizer` | Purge timeout enforcement |
| `operator.kyma-project.io/watcher` | Watcher cleanup |
| `operator.kyma-project.io/mandatory-module` | Mandatory module cleanup |

### Field Owners (for SSA)

| Owner | Context |
|-------|---------|
| `operator.kyma-project.io/lifecycle-manager` | Main field manager |
| `lifecycle-manager` | Legacy field manager |
| `declarative.kyma-project.io/applier` | Declarative resource applier |
| `catalog-sync` | Module catalog synchronization |
| `kyma-sync-context` | Kyma spec synchronization |

### Resource States

| State | Meaning |
|-------|---------|
| `Ready` | Successfully installed |
| `Processing` | Reconciling/installing |
| `Error` | Installation error |
| `Deleting` | Being deleted |
| `Warning` | Deployed but misconfigured |
| `Unmanaged` | Module unmanaged by operator |

### Namespaces

| Namespace | Purpose |
|-----------|---------|
| `kcp-system` | Control plane (KLM deployment, Kyma CRs) |
| `kyma-system` | SKR remote namespace (modules, catalog) |
| `istio-system` | Istio infrastructure, certificate issuers |

---

## 13. Configuration Flags

### Controller Concurrency

| Flag | Default | Description |
|------|---------|-------------|
| `--max-concurrent-kyma-reconciles` | 1 | Max concurrent Kyma reconciles |
| `--max-concurrent-manifest-reconciles` | 1 | Max concurrent Manifest reconciles |
| `--max-concurrent-watcher-reconciles` | 1 | Max concurrent Watcher reconciles |
| `--max-concurrent-mandatory-modules-reconciles` | 1 | Max concurrent mandatory install |
| `--max-concurrent-mandatory-modules-deletion-reconciles` | 1 | Max concurrent mandatory delete |

### Requeue Intervals

| Flag | Default | Description |
|------|---------|-------------|
| `--kyma-requeue-success-interval` | 5m | Kyma Ready state |
| `--kyma-requeue-error-interval` | 2s | Kyma Error state |
| `--kyma-requeue-warning-interval` | 30s | Kyma Warning state |
| `--kyma-requeue-busy-interval` | 5s | Kyma Processing state |
| `--manifest-requeue-success-interval` | 5m | Manifest Ready state |
| `--manifest-requeue-error-interval` | 2s | Manifest Error state |
| `--manifest-requeue-jitter-probability` | 0.02 | Jitter application probability |
| `--manifest-requeue-jitter-percentage` | 0.02 | Jitter percentage range |

### Kubernetes Client

| Flag | Default | Description |
|------|---------|-------------|
| `--k8s-client-qps` | 1000 | KCP client QPS |
| `--k8s-client-burst` | 2000 | KCP client burst |
| `--k8s-skr-client-qps` | 50 | SKR client QPS |
| `--k8s-skr-client-burst` | 100 | SKR client burst |

### Rate Limiting

| Flag | Default | Description |
|------|---------|-------------|
| `--rate-limiter-frequency` | 1000 | Requests/second |
| `--rate-limiter-burst` | 2000 | Burst size |
| `--failure-base-delay` | 5s | Base retry delay |
| `--failure-max-delay` | 30s | Max retry delay |
| `--cache-sync-timeout` | 60m | Cache sync timeout |

### Certificates

| Flag | Default | Description |
|------|---------|-------------|
| `--cert-management` | cert-manager.io/v1 | Certificate backend |
| `--self-signed-cert-duration` | 1440h (60d) | Certificate duration |
| `--self-signed-cert-renew-before` | 720h (30d) | Renewal trigger |
| `--self-signed-cert-renew-buffer` | 24h | Renewal confirmation wait |
| `--self-signed-cert-key-size` | 4096 | RSA key size (must be 4096) |

### Istio Gateway

| Flag | Default | Description |
|------|---------|-------------|
| `--istio-namespace` | istio-system | Istio namespace |
| `--istio-gateway-name` | klm-watcher | Gateway name |
| `--istio-gateway-namespace` | kcp-system | Gateway namespace |
| `--istio-gateway-server-cert-switch-grace-period` | 96h (4d) | Rotation grace period |
| `--istio-gateway-server-cert-expiry-window` | 336h (14d) | Pre-expiry window |

### SKR Watcher

| Flag | Default | Description |
|------|---------|-------------|
| `--skr-watcher-image-tag` | (REQUIRED) | Watcher image tag |
| `--skr-watcher-image-name` | runtime-watcher | Image name |
| `--skr-watcher-image-registry` | europe-docker.pkg.dev/kyma-project/prod | Registry |
| `--skr-webhook-memory-limits` | 200Mi | Webhook memory limit |
| `--skr-webhook-cpu-limits` | 0.1 | Webhook CPU limit |

### Other

| Flag | Default | Description |
|------|---------|-------------|
| `--purge-finalizer-timeout` | 5m | Purge deadline after deletion |
| `--min-maintenance-window-size` | 20m | Minimum maintenance window |
| `--module-upgrade-rollout-max-delay` | 5m | Random delay for version requeue |
| `--sync-namespace` | kyma-system | SKR sync namespace |
| `--metrics-cleanup-interval` | 15 | Minutes between metric cleanup |
| `--log-level` | -1 (Warn) | Log verbosity |
| `--leader-elect` | true | Enable leader election |
| `--pprof` | false | Enable profiling |

### Flag Validation Rules

- `WatcherImageTag` must be provided (no default)
- `LeaderElectionRenewDeadline` < `LeaderElectionLeaseDuration`
- `SelfSignedCertKeySize` must be exactly 4096
- `ManifestRequeueJitterProbability` must be 0 to 0.05
- `ManifestRequeueJitterPercentage` must be 0 to 1
- `CertificateManagement` must be `cert-manager.io/v1` or `cert.gardener.cloud/v1alpha1`
- Exactly one of `--oci-registry-host` or `--oci-registry-cred-secret` must be set

---

## 14. Metrics & Observability

### Prometheus Metrics (port 8080 at /metrics)

| Metric | Type | Description |
|--------|------|-------------|
| `lifecycle_mgr_requeue_reason_total` | Counter | Requeue reason tracking |
| `lifecycle_mgr_kyma_state` | Gauge | Kyma CR state |
| `lifecycle_mgr_module_state` | Gauge | Module state |
| `lifecycle_mgr_mandatory_modules` | Gauge | Mandatory modules count |
| `reconcile_duration_seconds` | Histogram | Manifest reconciliation duration |
| `lifecycle_mgr_purgectrl_time` | Histogram | Purge reconciliation duration |
| `lifecycle_mgr_self_signed_cert_not_renew` | Gauge | Certificate renewal status |
| `lifecycle_mgr_maintenance_window_*` | Various | Maintenance window metrics |

### Grafana Dashboards

1. **klm-dashboard-overview** — Performance and reconciliation metrics
2. **klm-dashboard-status** — Kyma CR and module state tracking
3. **klm-dashboard-watcher** — Watcher deployment and request metrics
4. **klm-dashboard-mandatory-modules** — Mandatory module installation status

### Health Probes

- **Liveness:** `/healthz` on port 8081 (initial: 15s, period: 20s)
- **Readiness:** `/readyz` on port 8081 (initial: 5s, period: 10s)

### Profiling

- pprof available on port 8084 when `--pprof=true`
- Endpoints: `/debug/pprof/`, profile, symbol, trace, cmdline

---

## 15. Testing Infrastructure

### Test Levels

| Level | Framework | Location | Scope |
|-------|-----------|----------|-------|
| Unit | Testify + Go testing | Throughout source | Individual functions |
| Integration | Ginkgo v2 + envtest | `tests/integration/` | Controllers with K8s API |
| E2E | Ginkgo v2 + k3d | `tests/e2e/` | Full system (KCP + SKR) |

### Unit Tests (~170 files)

- Distributed throughout source packages
- Coverage enforced per-package via YAML configs:
  - Repositories: 95-100%
  - Services: 71-100%
  - Internal packages: 82-100%
  - UI/manifest layers: 10-25%
- All commands include `GOFIPS140=v1.0.0`

### Integration Tests (32 files)

- Uses controller-runtime's `envtest` (K8s 1.32.0)
- Tests controllers against real K8s API server (in-process)
- Organized by component: `controller/kyma/`, `controller/modulereleasemeta/`, `watcher/`

### E2E Tests (41 files, 40+ scenarios)

- k3d clusters (KCP + SKR)
- 20-minute timeout per test
- 10 flake retry attempts
- Matrix strategy in CI for parallel execution
- Scenarios include: module lifecycle, deletion, upgrade, watcher, RBAC, certificates, maintenance windows

#### E2E Makefile Structure — Two Approaches

The E2E tests have **two approaches** for defining and running tests:

**OLD Approach (being phased out):** Direct `go test` calls in the main `tests/e2e/Makefile`. Assumes clusters are pre-created and KLM is pre-deployed externally (setup hidden in `.github/actions/`).

```makefile
# Example of OLD approach in tests/e2e/Makefile:
kyma-metrics:
    $(GO_TEST) "Manage Module Metrics"
```

Where `GO_TEST := GOFIPS140=$(FIPS140_MODULE_VERSION) go test -timeout 20m -ginkgo.v -ginkgo.focus`

**NEW Approach (preferred, being adopted):** Separate `.mk` file per test that defines the full lifecycle including cluster creation, KLM patching, module setup, and test execution. This makes tests fully self-contained and runnable locally without needing CI actions.

```makefile
# Example of NEW approach in tests/e2e/Makefile:
mandatory-module:
    $(MAKE) -f $(MAKEFILE_DIR)/mandatory_module_test.mk test
```

#### NEW Approach: `.mk` File Structure

Each test gets its own `.mk` file based on the template `e2e_test_template.mk`. The file defines:

1. **`klm-patch`** — Test-specific KLM configuration patches (kustomize patches to deployment args)
2. **`module-setup`** — Test-specific module template/metadata deployment
3. **`test-run`** — The actual Ginkgo test execution with focus string
4. **`test`** — Chains them: `test: create-clusters klm-patch deploy-klm module-setup test-run`

**Key files:**
| File | Purpose |
|------|---------|
| `tests/e2e/e2e.common.mk` | Shared infrastructure: tool installation, cluster lifecycle, module setup targets, variables |
| `tests/e2e/e2e_test_template.mk` | Template to copy when creating a new test .mk file |
| `tests/e2e/gateway_secret_server_cert_metric_test.mk` | Example: minimal (KLM patch, no module setup) |
| `tests/e2e/mandatory_module_test.mk` | Example: complex (multiple module versions + MRM) |
| `tests/e2e/mandatory_modules_metrics_test.mk` | Example: medium (single mandatory module + MRM) |

#### e2e.common.mk — Shared Infrastructure

**Variables:**
```makefile
MODULE_NAME                       := template-operator
MODULE_DEPLOYABLE_VERSION         := 1.0.4
MODULE_DEPLOYMENT_CURRENT_VERSION := template-operator-controller-manager
MODULE_OLDER_VERSION              := 1.1.0-smoke-test
MODULE_DEPLOYMENT_OLDER_VERSION   := template-operator-v1-controller-manager
MODULE_NEWER_VERSION              := 2.4.1-smoke-test
MODULE_DEPLOYMENT_NEWER_VERSION   := template-operator-v2-controller-manager
GO                                := GOFIPS140=v1.0.0 go
```

**Shared targets:**
- `tools-install` — Install all CLI tools (ginkgo, kustomize, istioctl, modulectl, ocm) to `tests/e2e/bin/`
- `create-clusters` — Create k3d KCP + SKR clusters with Istio and cert-manager
- `deploy-klm` — Deploy KLM from sources into KCP cluster
- `teardown` — Delete KCP and SKR clusters
- `log-tool-versions` — Print all tool versions for debugging

**Module setup targets (staging):**
- **`module-setup-latest`** — Deploy template-operator v1.0.4 (current version)
- **`module-setup-in-older-version`** — Deploy template-operator v1.1.0-smoke-test (older)
- **`module-setup-in-newer-version`** — Deploy template-operator v2.4.1-smoke-test (newer)

These call `scripts/tests/deploy_moduletemplate_e2e.sh` with appropriate flags. Tests that need custom module setup (e.g., mandatory modules, multiple versions) implement their own `module-setup` target instead.

#### Full Test Lifecycle (New Approach)

```
make -f tests/e2e/<testname>_test.mk test

1. create-clusters
   ├── Create SKR k3d cluster (ports 10080, 10443, 2112)
   ├── Create KCP k3d cluster (ports 9443, 9080, 9081, local registry :5111)
   ├── Install Istio on KCP (demo profile)
   ├── Install cert-manager on KCP
   ├── Configure CoreDNS for SKR host resolution
   └── Export kubeconfigs to ~/.k3d/

2. klm-patch (test-specific)
   └── e.g., kustomize edit add patch to KLM deployment args

3. deploy-klm
   ├── Apply kustomize patches (oci-registry-host, etc.)
   ├── Deploy KLM manifests to kcp-system
   ├── Wait for KLM pod ready
   └── Patch metrics endpoint

4. module-setup (test-specific)
   ├── Deploy ModuleTemplates (older/newer/mandatory versions)
   └── Deploy ModuleReleaseMetas (channel assignments or mandatory)

5. test-run
   ├── Export KCP_KUBECONFIG and SKR_KUBECONFIG
   └── go test -timeout 20m -ginkgo.v -ginkgo.focus "<Describe block name>"
```

#### Example: Creating a New E2E Test .mk File

```makefile
.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
    @echo "::group::KLM patch"
    @echo "No test-specific KLM patches"
    @echo "::endgroup::"

.PHONY: module-setup
module-setup: module-setup-latest
    # Or: module-setup-in-older-version, module-setup-in-newer-version
    # Or: custom setup with deploy_moduletemplate_e2e.sh / deploy_mandatory_modulereleasemeta.sh

.PHONY: test-run
test-run: log-tool-versions
    @echo "::group::Setting kubeconfig variables"
    @export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
    @export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
    @echo "::endgroup::"
    @echo "::group::E2E test: <Test Name>"
    @export PATH=$(LOCALBIN):$$PATH
    @pushd $(E2E_TESTS_DIR) > /dev/null
    set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "<Ginkgo Describe String>"; status=$$?; set -e
    @popd > /dev/null
    @echo "::endgroup::"
    exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
```

#### Important: Determining Module Setup for a Test

If unsure which module setup a test requires (`module-setup-latest`, `module-setup-in-older-version`, `module-setup-in-newer-version`, or a custom setup), **ask the user before creating the .mk file**. The setup depends on what the test's Go code expects — some tests assume a specific module version is pre-deployed, some need both older and newer versions, and some need mandatory module metadata.

### Test Utilities

- `pkg/testutils/builder/` — Object builders (Kyma, Manifest, ModuleTemplate, etc.)
- `tests/e2e/commontestutils/` — E2E helpers (deployment, metrics, RBAC, watcher)
- `tests/integration/commontestutils/` — Integration test helpers
- `.mockery.yaml` — Mock generation for interfaces

### Makefile Targets

```
make test                         # Full suite (unit + integration)
make unittest-klm                 # KLM unit tests with coverage
make unittest-api                 # API module unit tests
make unittest-maintenancewindows  # Maintenance windows tests
make lint                         # golangci-lint (all 3 modules)
```

---

## 16. CI/CD Pipelines

### GitHub Workflows (21 total)

**Testing:**
- `test-unit.yml` — Unit + integration on PR
- `test-e2e.yml` — Full E2E suite (40+ matrix jobs)
- `test-e2e-with-gcm.yml` — E2E with Gardener CertManager

**Quality Gates:**
- `verify-unit-test-coverage.yml` — Coverage per package
- `check-test-changes.yml` — Enforces test changes with code changes
- `check-api-compatibility.yml` — CRD backward compatibility (dyff/yq)
- `lint-golangci.yml` — golangci-lint on 3 modules
- `lint-yaml.yml` — YAML linting
- `lint-markdown-links.yml` — Markdown link validation
- `lint-conventional-prs.yml` — Conventional commits

**Build & Release:**
- `build-image.yml` — Docker image on main
- `build-image-local.yml` — Image for PR testing
- `create-release-klm.yml` — Lifecycle-manager release
- `create-release-api.yml` — API module release
- `create-release-maintenancewindows.yml` — Maintenance windows release

**Reporting:**
- `report-acceptance-criteria.yml` — From test DSL
- `report-package-metrics.yml` — Package metrics
- `report-sprint-commits.yml` — Sprint tracking
- `scorecard.yml` — OSSF security scorecard

### E2E Infrastructure

Custom GitHub Actions in `.github/actions/`:
- Cluster setup (k3d KCP + SKR)
- Tool installation (Go, kubectl, k3d, kustomize, istioctl, modulectl, ocm-cli)
- KLM deployment
- Template-operator deployment
- Private registry setup
- Network configuration (hostname patching, CoreDNS)

---

## 17. Build & Deployment

### Build

```bash
make build          # Binary: bin/manager (Go 1.26.2, FIPS140)
make docker-build   # Image: europe-docker.pkg.dev/kyma-project/prod/lifecycle-manager
make manifests      # Generate CRDs, RBAC, WebhookConfig
make generate       # Generate DeepCopy methods
```

### Deployment

- **Namespace:** `kcp-system`
- **Deployment:** `klm-controller-manager` (1 replica)
- **Resources:** 10m CPU request, 64Mi memory request, 1024Mi memory limit
- **Leader Election ID:** `893110f7.kyma-project.io`
- **Security:** runAsNonRoot, no privilege escalation
- **Termination Grace Period:** 10 seconds

### Kustomize Structure

```
config/
├── control-plane/     # Full deployment kustomization
├── manager/           # Controller deployment
├── crd/               # CRD manifests (5 CRDs)
├── rbac/              # RBAC (ServiceAccount, ClusterRoles, Roles)
├── istio/             # Istio Gateway and configuration
├── watcher/           # Watcher component
├── webhook/           # Admission webhooks
├── grafana/           # Dashboard ConfigMaps
└── certmanager/       # Certificate management
```

### RBAC Summary

The controller requires permissions for:
- Core resources: configmaps, events, secrets, services
- Istio: gateways (read), virtualservices (full CRUD)
- KLM CRDs: kymas, manifests, moduletemplates, modulereleasemetas, watchers (full CRUD + status + finalizers)
- API extensions: customresourcedefinitions (read + status)
- Leader election: configmaps, leases (full CRUD)

---

## 18. Architecture Decision Records (ADRs)

| ADR | Title | Key Decision |
|-----|-------|--------------|
| 000 | ADR Format | Decisions documented as ADRs in `/docs/contributor/adr` |
| 001 | Consumer-Defined Interfaces | Interfaces defined at consumer side (implicit fulfillment) |
| 002 | Constructor Injection | Dependencies via `New<struct>` constructors; composition functions at `cmd/` |
| 003 | Client Scope | `client.Client` only in Repository layer; prefer sub-interfaces |
| 004 | Layered Architecture | 3-tier: Controller → Service → Repository (downward deps only) |
| 005 | Consistent Naming | Types suffixed as controllers/services/repositories; no Interface/Impl |
| 006 | CRD Upgrade Strategy | Track CRD generations in annotations; detect diffs between KCP/SKR |
| 007 | PKI Certificates | CA-only PKI; 6-step zero-downtime rotation process |
| 008 | Unstructured SSA | Server-Side Apply for module resources; no preceding reads |

---

## 19. Key Design Patterns

### Patterns Used

| Pattern | Where | Purpose |
|---------|-------|---------|
| State Machine | Kyma/Manifest controllers | Lifecycle transitions |
| Ordered Use Cases | Deletion services | Strict sequential execution |
| Decorator | MaintenanceWindowDecorator | Adds behavior without modifying lookup |
| Strategy | ModuleTemplateInfoLookupStrategy | Pluggable lookup logic |
| Repository | internal/repository/ | Data access abstraction |
| Factory | SkrContextProvider | SKR client creation |
| Observer/Watch | internal/watch/ | Template/MRM change propagation |
| Proxy | SKR Client proxy | REST mapper verification |
| Builder | pkg/testutils/builder/ | Test object construction |
| Options | Service constructors | Testable optional dependencies |

### Caching Strategies

| What | Where | TTL | Strategy |
|------|-------|-----|----------|
| SKR Kubernetes clients | internal/remote/client_cache.go | 23-25h (random) | Prevents thundering herd |
| Component descriptors | internal/descriptor/cache/ | Runtime | sync.Map |
| Parsed manifests | internal/declarative/v2/ | 24h | In-memory with eviction |
| REST clients | internal/service/skrclient/ | Per SKR client lifetime | sync.Map (thread-safe) |
| CRD metadata | internal/remote/crd_upgrade.go | Per reconciliation | Annotation-based |

### Error Handling

- Typed errors per domain (internal/errors/)
- Error classification: connection vs authorization vs not-found
- Graceful degradation on SKR connection issues (cache invalidation)
- `errors.Join` for aggregating concurrent errors
- Wrapped errors for context preservation (`fmt.Errorf("context: %w", err)`)

### Concurrency

- Goroutines for module manifest reconciliation
- Concurrent catalog sync workers (apply + delete)
- Error channels for collecting goroutine failures
- `sync.Mutex` for REST client caches
- Rate limiting (base/max delay with burst)
- Jitter on requeue intervals (probabilistic)

---

## 20. Directory Structure

```
kyma-lifecycle-manager/
├── api/                              # API module (separate go.mod)
│   ├── v1beta1/                      # Deprecated API version
│   ├── v1beta2/                      # Current storage version
│   │   ├── kyma_types.go            # Kyma CRD types
│   │   ├── manifest_types.go        # Manifest CRD types
│   │   ├── moduletemplate_types.go   # ModuleTemplate CRD types
│   │   ├── modulereleasemeta_types.go # MRM CRD types
│   │   ├── watcher_types.go         # Watcher CRD types
│   │   └── condition_messages.go    # Condition constants
│   └── shared/                       # Shared types, labels, annotations
│       ├── operator_labels.go
│       ├── operator_annotations.go
│       ├── cr_finalizer.go
│       ├── cr_kind.go
│       ├── state.go
│       ├── namespace.go
│       └── istio.go
├── cmd/
│   └── main.go                       # Entry point, controller registration
├── config/                           # Kubernetes manifests
│   ├── control-plane/                # Full deployment kustomization
│   ├── crd/bases/                    # Generated CRD YAMLs
│   ├── manager/                      # Controller deployment
│   ├── rbac/                         # RBAC configuration
│   ├── watcher/                      # Watcher resources
│   ├── istio/                        # Istio Gateway/VirtualService
│   ├── webhook/                      # Admission webhooks
│   └── grafana/                      # Dashboard ConfigMaps
├── docs/
│   ├── contributor/
│   │   ├── adr/                      # Architecture Decision Records
│   │   ├── 01-architecture.md
│   │   ├── 02-controllers.md
│   │   ├── 08-kcp-skr-synchronization.md
│   │   ├── 10-maintenance-windows.md
│   │   ├── 11-components.md
│   │   └── 12-klm-arguments.md
│   ├── user/                         # End-user docs
│   └── operator/                     # Operator docs
├── internal/
│   ├── controller/                   # Controller implementations
│   │   ├── kyma/                     # Kyma controller + setup + SKR event handler
│   │   ├── mandatorymodule/          # Mandatory module controllers
│   │   ├── purge/                    # Purge controller
│   │   ├── watcher/                  # Watcher controller
│   │   ├── istiogatewaysecret/       # Gateway secret controller
│   │   └── manifest/                 # Manifest controller setup
│   ├── service/                      # Business logic layer
│   │   ├── kyma/deletion/            # 11-step deletion workflow
│   │   ├── kyma/status/              # Status management
│   │   ├── mandatorymodule/          # Mandatory module services
│   │   ├── restrictedmodule/         # Restricted module matching
│   │   ├── skrclient/                # SKR client management
│   │   ├── skrsync/                  # SKR synchronization
│   │   ├── componentdescriptor/      # OCM descriptor service
│   │   ├── accessmanager/            # Access credential management
│   │   └── watcher/                  # Watcher services + certificate
│   ├── repository/                   # Data access layer
│   │   ├── kyma/                     # Kyma repository
│   │   ├── manifest/                 # Manifest repository
│   │   ├── moduletemplate/           # ModuleTemplate repository
│   │   ├── modulereleasemeta/        # MRM repository
│   │   ├── secret/                   # Secret repository
│   │   ├── ocm/                      # OCI/OCM repository
│   │   └── skr/                      # SKR-specific repositories
│   ├── declarative/v2/               # SSA reconciliation engine
│   ├── remote/                       # Remote cluster management
│   ├── manifest/                     # Manifest handling utilities
│   │   ├── manifestclient/           # Manifest K8s client
│   │   ├── status/                   # Manifest status/conditions
│   │   ├── statecheck/              # Deployment/StatefulSet readiness
│   │   ├── parser/                   # Template parsing
│   │   └── finalizer/               # Finalization logic
│   ├── descriptor/                   # OCM descriptor cache/provider
│   ├── watch/                        # Event handlers (template/MRM changes)
│   ├── maintenancewindows/           # Maintenance window logic
│   ├── imagerewrite/                 # Docker image rewriting
│   ├── istio/                        # Istio VirtualService client
│   ├── gatewaysecret/                # Gateway cert management
│   ├── event/                        # K8s event recording
│   ├── result/                       # Use case results + events
│   ├── errors/                       # Typed error definitions
│   ├── common/                       # Shared constants
│   │   ├── fieldindex/               # Field indexing
│   │   └── fieldowners/              # SSA field owners
│   ├── crd/                          # CRD version management
│   ├── util/                         # Collections, diffing
│   └── pkg/                          # Internal utilities
│       ├── metrics/                  # Prometheus metrics
│       ├── flags/                    # CLI flag definitions
│       └── resources/                # Resource cleanup
├── pkg/                              # Public packages
│   ├── log/                          # Structured logging (Zap)
│   ├── status/                       # Kyma status management
│   ├── queue/                        # Requeue intervals + jitter
│   ├── templatelookup/               # Template resolution
│   │   └── moduletemplateinfolookup/ # Lookup strategies + decorators
│   ├── module/
│   │   ├── common/                   # Module types
│   │   └── sync/                     # Manifest sync runner
│   ├── matcher/                      # Label matching
│   ├── util/                         # Error classification
│   ├── testutils/                    # Test builders and utilities
│   │   └── builder/                  # Object builders
│   ├── watcher/                      # Watcher utilities
│   └── common/                       # Common errors
├── maintenancewindows/               # Separate go module for MW resolver
├── tests/
│   ├── integration/                  # Integration tests (envtest)
│   ├── e2e/                          # E2E tests (k3d)
│   └── fixtures/                     # Test certificates
├── scripts/                          # Build and test scripts
│   └── tests/                        # E2E test scripts
├── .github/
│   ├── workflows/                    # 21 CI/CD workflows
│   └── actions/                      # Custom GitHub Actions
├── Makefile                          # Build targets
├── go.mod                            # Go module (FIPS140 enabled)
├── versions.yaml                     # Tool versions
├── .golangci.yaml                    # Linting configuration
└── .mockery.yaml                     # Mock generation config
```

---

## 21. Dependencies

### Core Dependencies

| Dependency | Version | Purpose |
|-----------|---------|---------|
| sigs.k8s.io/controller-runtime | v0.23.3 | Kubernetes operator framework |
| ocm.software/ocm | v0.40.0 | Open Component Model |
| github.com/kyma-project/runtime-watcher/listener | v1.4.0 | SKR event watching |
| istio.io/client-go | v1.29.2 | Istio API client |
| github.com/cert-manager/cert-manager | v1.20.2 | Certificate management |
| github.com/gardener/cert-management | v0.22.0 | Gardener certificates |
| github.com/prometheus/client_golang | v1.23.2 | Prometheus metrics |
| go.uber.org/zap | v1.28.0 | Structured logging |
| github.com/onsi/ginkgo/v2 | v2.28.2 | BDD testing |
| github.com/onsi/gomega | v1.39.1 | Test matchers |
| github.com/stretchr/testify | v1.11.1 | Test assertions |

### Local Module Replacements

```
github.com/kyma-project/lifecycle-manager/api => ./api
github.com/kyma-project/lifecycle-manager/maintenancewindows => ./maintenancewindows
```

### Go Version

Go 1.26.2 with `GOFIPS140=v1.0.0` (FIPS 140 compliance)

---

## 22. Tool Versions

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.26.1 | Language runtime |
| controller-gen | 0.18.0 | CRD/RBAC generation |
| kustomize | 5.4.3 | Manifest customization |
| golangci-lint | 2.9.0 | Code linting |
| k3d | 5.8.3 | Local K8s clusters |
| kubectl | 1.31.3 | K8s CLI |
| istioctl | 1.26.4 | Istio management |
| modulectl | 3.1.0 | Module CLI |
| ocm-cli | 0.32.0 | OCM operations |
| cert-manager | 1.19.3 | Certificate management |
| envtest | 0.21 | Integration test K8s |
| envtest K8s | 1.32.0 | Integration test K8s version |
| K8s (e2e) | 1.32.2 | E2E test K8s version |
| Docker | 27.5.1 | Container builds |

### Linting Rules (golangci-lint)

- **Complexity:** cyclop max-complexity: 20
- **Function length:** funlen lines: 80
- **Line length:** revive line-length-limit: 120
- **Nesting:** nestif min-complexity: 6
- **Import aliases:** Extensive configuration for consistent naming
- **Disabled linters:** contextcheck, depguard, exhaustruct, nlreturn, paralleltest, lll, sqlclosecheck, wsl
- **Test exclusions:** dupword, err113, funlen, gochecknoglobals, wrapcheck, varnamelen
- **Import order:** standard → default → kyma-project prefix → blank → dot

---

## Quick Reference: Entry Points

| What | Where |
|------|-------|
| Application start | `cmd/main.go` |
| Flag definitions | `internal/pkg/flags/flags.go` |
| Kyma reconcile | `internal/controller/kyma/controller.go:Reconcile()` |
| Manifest reconcile | `internal/declarative/v2/reconciler.go:Reconcile()` |
| Deletion workflow | `internal/service/kyma/deletion/deletion_service.go:Delete()` |
| Template lookup | `pkg/templatelookup/regular.go:GetRegularTemplates()` |
| Module sync | `pkg/module/sync/runner.go:ReconcileManifests()` |
| Remote catalog | `internal/remote/remote_catalog.go:SyncModuleCatalog()` |
| CRD sync | `internal/remote/crd_upgrade.go:Execute()` |
| Maintenance check | `internal/maintenancewindows/maintenance_window.go:IsRequired()/IsActive()` |
| Metrics setup | `internal/pkg/metrics/` |
| SKR client factory | `internal/service/skrclient/skr_client.go:ResolveClient()` |

---

## 23. Incident Debugging — Cluster Access

When investigating production incidents, KLM logs alone are usually not enough — you need direct access to both the **KCP** cluster (where KLM, Kyma CRs, and Manifest CRs live) and the affected **SKR** cluster (where the module operators and their CRs live). The two clusters use different authentication paths.

`<env>` below is one of `dev`, `stage`, or `prod` — pick the environment that matches the incident.

### 23.1 SKR Cluster Access (`kcp-cli`)

SKR clusters are managed via Gardener and accessed through the `kcp-cli` tool. Steps:

1. **Point `KCPCONFIG` at the right environment config**
   ```bash
   export KCPCONFIG="$HOME/.kcp-cli/config-<env>.yaml"
   ```
   If login fails because the config is outdated, fresh configs are published at:
   <https://github.tools.sap/kyma/documentation/tree/main/docs/kyma-internal/support/on-call-guides/kcp/assets>

2. **Interactive login** — opens a browser window for SSO. The user must complete this step manually:
   ```bash
   kcp-cli login
   ```

3. **Fetch the SKR kubeconfig by shoot name**
   ```bash
   kcp-cli kubeconfig -c <shoot>
   ```
   `<shoot>` is the cluster's shoot identifier (e.g., `d3c1d4b` from incident alerts). This sets `KUBECONFIG` to point at the SKR.

### 23.2 KCP Cluster Access (`gcloud`)

KCP clusters run on GKE and are accessed via `gcloud`. Steps:

1. **Interactive login** — the user must authenticate manually:
   ```bash
   gcloud auth login
   ```

2. **Fetch credentials for the KCP GKE cluster**
   ```bash
   gcloud container clusters get-credentials kyma-mps-<env> \
     --region europe-west1 \
     --project sap-ti-dx-kyma-mps-<env>
   ```
   After this, `kubectl` operates against the KCP cluster.

### 23.3 Typical Debugging Workflow

For a "module X stuck / not ready on SKR Y" incident:

| Step | Cluster | Command (examples) | Looking for |
|------|---------|--------------------|-------------|
| 1 | KCP | `kubectl -n kcp-system get kyma <runtime-id>` | Kyma CR aggregated state, per-module status, conditions |
| 2 | KCP | `kubectl -n kcp-system get manifest -l operator.kyma-project.io/kyma-name=<runtime-id>,operator.kyma-project.io/module-name=<module>` | Manifest state, synced resources, conditions |
| 3 | KCP | `kubectl -n kcp-system logs deploy/klm-controller-manager` | KLM reconciler errors |
| 4 | SKR | `kubectl get <module-cr-kind> -A -o yaml` | Module CR existence and status (e.g., `Istio`, `BtpOperator`) |
| 5 | SKR | `kubectl -n kyma-system get deploy,sts,pods` | Module manager (operator) workload health |
| 6 | SKR | `kubectl -n kyma-system logs <module-operator-deploy>` | Module operator reconcile errors |
| 7 | SKR | `kubectl -n istio-system get deploy,svc,gateway,virtualservice` | Istio infrastructure (relevant for ingress/healthz incidents) |

### 23.4 Tips

- Always confirm `customResourcePolicy` for the affected module in the Kyma CR — `Ignore` means KLM does **not** create the module's CR; missing CR is then expected from KLM's perspective.
- Manifest `state: Ready` only reflects **manager Deployment/StatefulSet** readiness (`internal/manifest/statecheck/`); it does not validate the module CR or the resources the module operator should produce.
- The synthetic error `manifest state requires update: from <X> to <Y>` (`internal/declarative/v2/reconciler.go:517,525`) is a control-flow signal for state transitions, not a real failure — ignore it during triage unless it loops indefinitely.
- For SKR connectivity issues from KLM, check the access secret on KCP: `kubectl -n kcp-system get secret -l operator.kyma-project.io/kyma-name=<runtime-id>`.
