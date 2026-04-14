# ADR 008 - Use Unstructured Server-Side Apply for Syncing Module Resources to Kyma Runtimes

## Status

Accepted

## Context

Lifecycle-Manager already uses Server-Side Apply (SSA) for syncing the module resources extracted from the module's raw manifest to the Kyma Runtimes. This ADR serves as documentation on why SSA is used and how it is implemented for this task.

### Background

Starting from the `kubectl` perspective, SSA (`kubectl apply --server-side`) is an alternative to Client-Side Apply (CSA `kubectl apply`). The key differences between the two are the following:

- CSA requires the user to specify a full object. Under the hood, CSA performs a three-way strategic merge between the last-applied configuration (stored in the `kubectl.kubernetes.io/last-applied-configuration` annotation), the live state on the server, and the new desired state.
- SSA allows the user to send a partial object. The API server takes care of combining this partial object with the existing object, if any. For conflict management, SSA tracks *managed fields* instead of optimistic locking. Only if trying to change a field owned by another manager, a conflict may be raised where an override can be forced.

Looking at it from controller-runtime perspective, the approach for SSA used to be providing a `client.Apply` patch to the `client.Patch()` function. One of the major problems with this approach stems from Go's zero values and JSON marshalling: fields without omitempty in their JSON tags are always marshalled, even if the caller never intended to set them — potentially overwriting existing values with zero values. Conversely, fields with omitempty are silently dropped when set to zero/false/nil, even if the caller explicitly wants to set them to that value. The `ApplyConfiguration` approach introduced recently with the [native `client.Apply() function`](https://github.com/kubernetes-sigs/controller-runtime/issues/3183) solves both problems: every field is a pointer, so nil unambiguously means "omit this field" and a non-nil pointer unambiguously means "include this field with this value", regardless of whether the value happens to be zero. `ApplyConfigurations` are available for known K8s resources in [`client-go/applyconfigurations`](https://pkg.go.dev/k8s.io/client-go@v0.35.3/applyconfigurations). For custom types, they can be generated, e.g. via `controller-gen` or specifically [`applyconfiguration-gen`](https://pkg.go.dev/k8s.io/code-generator/cmd/applyconfiguration-gen). In addition, unstructured objects can be converted to `ApplyConfigurations` by `controller-runtime`s `client.ApplyConfigurationFromUnstructured()`.

Not using SSA, controller-runtime provides the lower level primitives `client.Get()`, `client.Create()`, `client.Update()`, and `client.Patch()`. Important to consider is that a `client.Update()` is a full replacement of the object with optimistic locking based on `.metadata.resourceVersion`, hence it requires a read of the object before the update. `client.Patch()` on the other hand patches the object from partial information. The actual patch can be constructed manually (raw patch), or computed on the client based on different strategies considering current and new object. Unlike `client.Update()`, `client.Patch()` does not enforce optimistic locking by default, so no up-to-date `.metadata.resourceVersion` is required. In addition to the above, there are the [`controllerutil.CreateOrUpdate()`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/controller/controllerutil#CreateOrUpdate) and [`controllerutil.CreateOrPatch()`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/controller/controllerutil#CreateOrPatch) helpers.

See also [Kubernetes Controllers at Scale: Clients, Caches, Conflicts, Patches Explained](https://medium.com/@timebertt/kubernetes-controllers-at-scale-clients-caches-conflicts-patches-explained-aa0f7a8b4332) as an excellent source for more details.

## Decision

Syncing the module resources extracted from the module's raw manifest to the Kyma Runtimes is an extensive task. Because of the significant number of resources to be synced, this needs to run as efficient as possible. Running a `client.Update()` to fully overwrite the object requires a preceding `client.Get()` which is an uncached call over network. Running a `client.Patch()` typically also requires a preceding `client.Get()` as the diff is computed from the current and the target object. Using a `client.RawPatch()`, it would be possible to issue a `client.Patch()` without preceding `client.Get()` and diff computation at Lifecycle Manager. This would be close to running an SSA, but SSA has the additional advantages of upsert semantics and proper field ownership tracking. Both SSA and `client.Patch()` with `client.RawPatch()` have the limitation of not reverting any additional fields set from other actors. Further SSA makes the API server drop fields that are owned by Lifecycle Manager and not included in the request anymore. With `client.RawPatch()`, such fields remain unchanged which makes use of `client.RawPatch()` infeasible.

It is decided that Lifecycle Manager continues to use SSA for syncing Module Resources to Kyma Runtimes. The benefits from using SSA are a) no preceding read necessary; b) no diff computation necessary on Lifecycle Manager side; c) upsert semantic (no explicit Create necessary); d) proper field ownership management; e) previously managed fields are removed if not provided anymore. The drawback that SSA will not revert fields added by other actors is accepted.

As the types of the synced resources are unknown to Lifecycle Manager, the implementation of the SSA is straightforward using `controller-runtime`s `client.ApplyConfigurationFromUnstructured()`.

> Note: Gardener Resource Manager uses a [custom CreateOrUpdate](https://github.com/gardener/gardener/blob/405bd37b527cec5f1b388245ad743912af91fe11/pkg/controllerutils/update.go#L19) implementation. A key difference between Gardener Resource Manager and Lifecycle Manager is that Gardener Resource Manager typically manages one Shoot, while Lifecycle Manager manages thousands of Runtimes.

## Consequence

Lifecycle Manager has been updated already with [chore: Implement `client.Apply` in ProxyClient and use it in ConcurrentDefaultSSA](https://github.com/kyma-project/lifecycle-manager/pull/3165/changes)
