
## ADR 007 - Introduce Finite State Machine (FSM) Pattern for Kyma Controller Reconciliation

| Key Information | Value                                                                           |
| :--- |:--------------------------------------------------------------------------------|
| **Title** | Introduce Finite State Machine (FSM) Pattern for Kyma Controller Reconciliation |
| **Status** | Proposed                                                                        |
| **Target Component** | Kyma Controller                                                                 |

### 1. Context and Background

The **Kyma Lifecycle Manager (KLM)** is a **meta operator** responsible for coordinating and tracking the lifecycle of Kyma Modules. Operating within the Kyma Control Plane (KCP), KLM must reconcile not only locally but also across remote clusters (SAP BTP, Kyma runtime, or SKR).

The **Kyma Controller** deals with the introspection, interpretation, and status update of the **Kyma Custom Resource (CR)**. It aggregates the status of installed modules (`Manifest` CRs) into a single `State` field on the Kyma CR, which reflects the integrity of the Kyma installation.

The current system manages the lifecycle using discrete status values: `Error`, `Ready`, `Processing`, `Warning`, `Deleting`, and `Unmanaged`. The reconciliation logic utilizes a primary `switch` statement on `kyma.Status.State` within the `processKymaState` function to route control flow.

### 2. Problem Statement

The current state management, which relies on implicit conditional logic and branching return paths across a large reconciliation function (`r.reconcile`), introduces complexity and potential ambiguity in transition handling:

1.  **Complex, Parallel Operations:** The key states, such as `StateProcessing`, involve complex parallel operations managed by `errgroup.Group`, including reconciling manifests, syncing the module catalog, and reconciling SKR webhooks. Determining the *final* outcome (transition to `Ready`, `Warning`, or `Error`) based on the collective results of these parallel tasks is handled imperatively.
2.  **Ambiguous Transition Paths:** When an operation fails, the controller often relies on `r.requeueWithError` to update the status to `StateError` and requeue. However, the exact rules governing which state can transition to which other state (e.g., preventing a direct jump from `Ready` to `Deleting` without confirmation of remote deletion trigger) are embedded in imperative code rather than a clear model.
3.  **Rigid Sequential Cleanup:** The de-provisioning flow (`handleDeletingState`) requires a strict sequence of cleanup steps (remote webhook removal, remote catalog deletion, manifest cleanup, finalizer removal). Deviations or errors in this sequence can lead to resource leaks if not explicitly handled and strictly adhered to.

### 3. FSM Definition (Formalizing the Current Model)

A **Finite State Machine (FSM)** provides a highly structured pattern where the system's current behavior is dictated solely by its current state, and movement to the next state is governed only by predefined transitions triggered by specific events.

#### A. States (Vertices)

The FSM utilizes the existing, well-defined states of the Kyma CR:

*   `""` (Initial)
*   `shared.StateProcessing`
*   `shared.StateReady`
*   `shared.StateWarning`
*   `shared.StateError`
*   `shared.StateDeleting`
*   `shared.StateUnmanaged`

#### B. Events and Actions (Inputs and Outputs)

Events are the outcomes of actions performed within a state (e.g., reconciliation complete, remote deletion acknowledged, unauthorized access error). Actions are the dedicated functions executed upon entering or during a state.

| Event (Trigger) | Example Action (Output) | Current Source Logic |
| :--- | :--- | :--- |
| **Lifecycle Change Detected** | Transition to `StateProcessing` (from `Ready` or `Initial`). | `handleInitialState`, `handleProcessingState`. |
| **All Modules/Syncs Complete** | Transition to `StateReady` (from `Processing`). | `kyma.DetermineState()` returns `StateReady`. |
| **Remote Access Unauthorized** | Invalidate cache, set module statuses to `StateError`. | `apierrors.IsUnauthorized(err)` handling. |
| **Deletion Timestamp Found** | Trigger remote deletion; Transition to `StateDeleting`. | `r.deleteRemoteKyma` and status update. |

### 4. Benefits: Predictability and Reduced Complexity

Implementing a formal FSM pattern will provide immense value by making the Kyma controller's behavior **predictable** and substantially **reducing the complexity** inherent in managing remote, asynchronous cluster states.

#### A. Enhanced Predictability

Predictability in the Kyma lifecycle is crucial because the controller manages remote user environments (SKR clusters).

1.  **Guaranteed Transition Integrity:** The FSM replaces the implicit logic where various functions return errors that must be handled manually with a declarative transition table. This ensures that a state (e.g., `StateReady`) can **only move to valid subsequent states** (e.g., `Processing` upon module change, or `Deleting` upon marked deletion) and eliminates the possibility of unexpected state jumps (e.g., accidentally moving to `Ready` when critical cleanup tasks are incomplete).
2.  **Explicit Requeuing Behavior:** The Kyma Controller already uses specific requeue intervals based on the current state (e.g., `kyma-requeue-success-interval`, `kyma-requeue-error-interval`, `kyma-requeue-busy-interval`). By enforcing FSM transitions, the controller guarantees that every action concludes with a valid state, directly linking that state to a predictable and configured retry duration (`requeueInterval := queue.DetermineRequeueInterval(state, r.RequeueIntervals)`).
3.  **Auditable State History:** Since the FSM requires explicit event processing for any transition, tracing the exact sequence of events that led a Kyma CR to an `Error` or `Ready` state becomes trivial. This enhances observability and debugging, especially when correlating with existing `lifecycle_mgr_kyma_state` metrics.

#### B. Reduced Operational and Code Complexity

1.  **Decoupling Logic from Transitions:** The primary complexity driver is the intermingling of business logic (e.g., `reconcileManifests`, synchronization services) with error handling and status updates. The FSM strictly separates the two: handlers focus only on executing actions, and a separate, dedicated FSM core logic determines the transition based on the action's result (Success/Failure/Busy).
2.  **Formalizing Asynchronous Coordination:** Within `StateProcessing`, the current logic relies on `errgroup.Wait()` to manage the parallel reconciliation of core services (Manifests, Catalog Sync, Webhook Reconcile). The FSM formalizes the rule: **If `errgroup.Wait()` returns success, transition to `Ready` (or `Warning`); if it returns an error, transition to `Error`**. This prevents complex, nested error checks from governing the state decision.
3.  **Streamlined Deletion Safeguards:** The critical cluster cleanup logic (Purge Controller) and deletion flow (`handleDeletingState`) require strict execution order to prevent resource leaks (e.g., removing finalizers only after manifests are gone). The FSM enforces this sequencing by structuring the deletion phase into distinct, sequential sub-transitions, significantly clarifying which cleanup step is currently blocked or completed.

### 5. Proposed Solution

The Kyma Controller reconciliation logic (`r.reconcile` and `r.processKymaState`) should be refactored to delegate state management to a specialized FSM implementation:

1.  **FSM Library/Component:** Integrate a library or implement a localized component to manage the state graph, defining all permissible transitions between the six primary Kyma states.
2.  **Refactored Handlers:** The existing handlers (`handleInitialState`, `handleProcessingState`, `handleDeletingState`, etc.) will be retained but refactored to focus purely on executing the required actions (e.g., synchronization, reconciliation). They will return a codified event result (e.g., `EventReady`, `EventFailure`, `EventBusy`) instead of directly manipulating the Kyma status or calculating the next state.
3.  **Transition Enforcement:** A central FSM function consumes the current state and the returned event, calculates the next guaranteed valid state according to the transition map, and executes the mandatory status update (using functions like `r.updateStatus`) before setting the corresponding requeue interval.

By using the FSM pattern, the Kyma Controller transforms its reconciliation process from complex procedural code into a verifiable, declarative system, acting as a **railway switching yard** where the train (the Kyma CR) can only move along designated tracks (transitions) to reach clear destinations (states), preventing unexpected derailments (error states) or missed stopovers (cleanup steps).
