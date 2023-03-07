# Declarative Reconciliation Library

This library uses declarative reconciliation to perform resource synchronization in clusters.

The easiest way to explain the difference between declarative and imperative code, would be that imperative code focuses on writing an explicit sequence of commands to describe how you want the computer to do things, and declarative code focuses on specifying the result of what you want.

Thus, in the declarative library, instead of writing "how" the reconciler behaves, you instead describe "what" it's behavior should be like. Of course, that does not mean that the behavior has to be programmed, but instead that the declarative reconciler is built on a set of [declarative options](v2/options.go) that can be applied and result in changes to the reconciliation behavior of the actual underlying [reconciler](v2/reconciler.go).

The declarative reconciliation is strongly inspired by the [kubebuilder declarative pattern](https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern), however it brings it's own spin to it as it contains it's own form of applying and tracking changes after using a `rendering` engine, with different implementations:

## Core

The core of the reconciliation is compromised of two main components:
1. [A Client interface](v2/client.go), its [factory implementation](v2/factory.go) that is optimized for [caching Resource Mappings](v2/client_cache.go) of the API-Server and [Dynamic Lookup of Resources](v2/client_proxy.go) to avoid heavy reload of schemata
2. A generic [Reconciler](v2/reconciler.go) which delegates various parts of the resource synchronization based on the [object specification](v2/spec.go) and the [options for reconciliation](v2/options.go). It owns the central conditions maintained and reported in the [object status](v2/object.go).

While the client is the main subsystem of the `Reconciler` implementation, the `Reconciler` also redirects to other subsystems to achieve its goal:
- A `Renderer` which determines all resources to reconcile as a byte stream
- A `Converter` which converts the rendered resources into internal objects for synchronization
- A `ReadyCheck` which introduces more detailed status checks rather than `Exists/NotFound` to the synchronization process, allowing more fine-grained error reporting.
- A `Status` which is embedded in the reconciled object and which returns the state of the synchronization

For more details on the subsystems used within the main `Reconciler`, check out the sections below.

## Configuration of the Library via declarative Options

The declarative Reconciliation is especially interesting because it is configured with a set of pre-defined [options for reconciliation](v2/options.go).

An example configuration can look like this:

```golang
func ManifestReconciler(
	mgr manager.Manager, codec *v1beta1.Codec,
	checkInterval time.Duration,
) *declarative.Reconciler {
	return declarative.NewFromManager(
		mgr, &v1beta1.Manifest{},
		declarative.WithSpecResolver(
			internalv1beta1.NewManifestSpecResolver(codec),
		),
		declarative.WithCustomReadyCheck(internalv1beta1.NewManifestCustomResourceReadyCheck()),
		declarative.WithRemoteTargetCluster(
			(&internalv1beta1.RemoteClusterLookup{KCP: &declarative.ClusterInfo{
				Client: mgr.GetClient(),
				Config: mgr.GetConfig(),
			}}).ConfigResolver,
		),
		declarative.WithClientCacheKeyFromLabelOrResource(v1beta1.KymaName),
		declarative.WithPostRun{internalv1beta1.PostRunCreateCR},
		declarative.WithPreDelete{internalv1beta1.PreDeleteDeleteCR},
		declarative.WithPeriodicConsistencyCheck(checkInterval),
	)
}
```

These options include but are not limited to:
- Configuration of custom Readiness Checks that verify the integrity of the installation
- Configuration of the Cluster where the installation should be located in the end
- A resolver to translate from a custom API-Version (such as our `Manifest`) into the internal [object specification](v2/spec.go)
- Post/Pre Run Hooks to inject additional logic and side-effects before and after specific installation steps
- Consistency Check configurations determining frequency of reconciliation efforts.


## Rendering Engines

All renderer implementations must implement the [renderer interface](v2/renderer.go) and are initialized based on a given `RendererMode` that is available in the [object specification](v2/spec.go).

Currently all renderer options

- [HELM](v2/renderer_helm.go) is an optimized and specced-down version of the Helm Templating Engine to use with Charts
- [kustomize](v2/renderer_kustomize.go) is an embedded [krusty](https://pkg.go.dev/sigs.k8s.io/kustomize/api/krusty) kustomize implementation
- [raw](v2/renderer_raw.go) is an easy to use raw renderer that just passes through manifests
- [cached](v2/renderer_with_cache.go) which uses an existing renderer and passes rendered resources by levaring a file cache in order to save on reoccuring reconciliation times. Especially useful for long render times from libraries like `HELM` or `kustomize`

Every renderer reconciles in a particular order through the [renderer interface](v2/renderer.go):

1. Construction of the Renderer based on the [object specification](v2/spec.go)
2. Initializing of the Renderer through `Initializer(Object)`, setting up necessary status conditions or environment settings
3. Prerequisite Initialization, e.g. dependencies of the output ([HELM](v2/renderer_helm.go) uses this for CRD layer initialization)
4. Rendering of the Resources into a valid `.yaml` compliant byte array
5. Removal of the PreRequisites if explicitly desired (by default they are not removed unlike the standard resources, imagine a CRD uninstallation)

## Resource Conversion

To allow tracking the resources created by the `renderer`, every resource is converted to a generic [resource](v2/resource_converter.go), which translates all objects into a [k8s cli-runtime compliant resource represenation](https://pkg.go.dev/k8s.io/cli-runtime/pkg/resource#Info), which contains information about the object, its API Version and Mappings towards a specific API Resource of the API server.

### Resource Cluster Synchronization

All [create/update](v2/ssa.go) and [delete](v2/cleanup.go) cluster interactions of the library are done by leveraging highly concurrent [ServerSideApply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) implementations that are written to:
1. Always apply the latest available version of resources in the schema by using inbuilt schema conversions
2. Delegate as much compute to the api-server to reduce overall load of the controller even with several hundred concurrent reconciliations.
3. Use a highly concurrent process that is rather focusing on retrying an apply and failing early and often instead of introducing dependencies between different resources. The library will always attempt to reconcile all resources in parallel and will simply ask for a retry in case it is determined there is a missing interdependency (e.g. a missing CustomResourceDefinition that is applied in parallel, only leading to sucessful reconciliations in subsequent reconciliations).

### Resource Tracking

Every resource rendered by the [renderer](v2/renderer.go) is tracked through a set of fields in the [declarative status in the object](v2/object.go). This can be embedded in objects through implementing the [Object interface](v2/object.go), a superset of the [`client.Object` from controller-runtime](https://github.com/kubernetes-sigs/controller-runtime/blob/main/pkg/client/object.go).

The library will use this status to report important events and status changes by levaring the `SetStatus` method. One can choose to either directly embed the `Status` and to implement `GetStatus()` and `SetStatus()` on the object, or to use a conversion instead that translates the declarative status to a versioned API status.

Important Parts of the `Status` are:
- A `State` representing the overall state of the installation
- Various `Conditions` compliant with [KEP-1623: Standardize Conditions](https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/1623-standardize-conditions)
- `Synced`, a list of resources with Name/Namespace as well as a [GroupVersionKind from the kubernetes apimachinery](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#GroupVersionKind), which is used to track individual resources resulting from the `renderer`
- `LastOperation`, a combination of a message / timestamp that is always updated whenever the library reconciles the [object specification](v2/spec.go) and issues more details than the current state (e.g. detailed error messages or success details of a step during the reconciliation)

While all synchronized resources are tracked in the `Synced` list, they are regularly checked against and pruned or created newly based on the reconciliation interval provided through the [options for reconciliation](v2/options.go).

## Resource Readiness

While the deletion and creation of resources is quite straight-forward, oftentimes the readiness of a given resource cannot be determined purely by its `existence` but also by specific reporting states derived from the status of an object, for example a Deployment: just because the deployment exists, it does not mean the image could be pulled and the container started.

To combat this problem, we introduce the [`ReadyCheck` interface](v2/ready_check.go), a simple interface that can provide custom Readiness Evaluations after Resource Synchronization. By default we make use of inbuilt [readiness implementations from `HELM`](https://github.com/helm/helm/blob/main/pkg/kube/ready.go) as it contains a lot of widely accepted standards for Readiness checks, however we are planning to eventually switch that to a more generic and better solution once available.

Once synchronized all resources are passed to the readiness checker, and if determined as not ready, the `LastOperation` and the appropriate `State` and Conditions will evaluate to a `Processing` or `Error` state.
