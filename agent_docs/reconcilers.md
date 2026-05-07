# Reconciler patterns

## Reconciler struct

Every controller follows the same shape:

```go
type Reconciler struct {
    client.Client           // embedded — use r.Get, r.List, r.Update, r.Delete directly
    event.Event             // emit Kubernetes events via r.Warning / r.Normal
    queue.RequeueIntervals  // Success, Busy, Error, Warning durations from flags

    // dependencies as interfaces — never concrete types
    SkrContextFactory  remote.SkrContextProvider
    DeletionService    DeletionService
    SKRWebhookManager  SKRWebhookManager
    // ...
}
```

Concrete dependencies are wired in `cmd/composition/` using pure composer functions. If you add
a new service dependency, add it as an interface field here and add a composer call there.

## Reconcile entry point

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
```

The pattern is always:
1. Fetch the object (`r.Get`). Return `ctrl.Result{}, nil` on not-found.
2. Initialise conditions (`status.InitConditions`).
3. Check skip/deletion annotations.
4. Do work; update status via `r.updateStatus` or `r.updateStatusWithError`.
5. Return `ctrl.Result{RequeueAfter: interval}, err`.

## Kyma state machine

```
""  ──► Processing ──► Ready
                  ├──► Warning
                  ├──► Error  (returns ErrKymaInErrorState so rate limiter applies)
                  └──► Deleting ──► (finalizer removed, no requeue)
```

`processKymaState` switches on `kyma.Status.State`. Both `Error` and `Warning` fall through to
`handleProcessingState` — errors don't stop reconciliation; they just set a condition.

## Spec is read from SKR

The Kyma `spec` on KCP is **overwritten** at the start of every reconcile from the SKR-side copy:

```go
remote.ReplaceSpec(controlPlaneKyma, remoteKyma)
```

Do not write business logic that depends on the KCP spec persisting across calls without this
override. The remote Kyma is the user-facing API surface.

## Status updates

Always update status through the helpers — never call `r.Status().Update()` directly:

```go
r.updateStatus(ctx, kyma, shared.StateReady, "kyma is ready")
r.updateStatusWithError(ctx, kyma, err)
```

These call `status.Helper(r).UpdateStatusForExistingModules(...)`, which also fires a Kubernetes
event. Condition updates are done in place on the object (`kyma.UpdateCondition(type, status)`)
before calling the helper.

## Finalizers

The kyma finalizer constant is `shared.KymaFinalizer`. Add/check it with:

```go
controllerutil.ContainsFinalizer(obj, shared.KymaFinalizer)
controllerutil.AddFinalizer(obj, shared.KymaFinalizer)
controllerutil.RemoveFinalizer(obj, shared.KymaFinalizer)
```

Finalizers are added in `kyma.EnsureLabelsAndFinalizers()` and removed at the end of
`handleDeletingState`. After removing a finalizer, always call `r.Update(ctx, obj)` (not
`r.Status().Update`).

## Requeueing

Use `queue.DetermineRequeueInterval(state, r.RequeueIntervals)` for normal requeue intervals.
Short explicit intervals (e.g. `1 * time.Second`) are only used during deletion transitions
where the next step is expected to be fast.

Returning a non-nil error triggers the controller-runtime rate limiter. Only return an error when
the condition is transient and should be rate-limited. For permanent/expected states (e.g. module
not yet ready), return `ctrl.Result{RequeueAfter: interval}, nil`.

## SKR context lifecycle

```go
r.SkrContextFactory.Init(ctx, kyma.GetNamespacedName())  // fetch/refresh credentials
skrCtx, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
// on auth errors:
r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
```

Always invalidate the cache on `apierrors.IsUnauthorized` or connection-related errors before
returning. The next reconcile will re-establish the client.

## Parallel operations in handleProcessingState

`handleProcessingState` uses `errgroup.Group` to fan out:
- `reconcileManifests` (module state)
- `RemoteCatalog.SyncModuleCatalog` (module catalog on SKR)
- `SKRWebhookManager.Reconcile` (watcher webhook on SKR)

These run concurrently. If any returns an error, `errGroup.Wait()` returns it and the reconcile
sets state to Error. Add new concurrent operations as new `errGroup.Go(func() error { ... })`.
