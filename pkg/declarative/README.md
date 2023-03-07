# Declarative Reconciliation Library

This library uses declarative reconciliation to perform resource synchronization in clusters.

The easiest way to explain the difference between declarative and imperative code, would be that imperative code focuses on writing an explicit sequence of commands to describe how you want the computer to do things, and declarative code focuses on specifying the result of what you want.

Thus, in the declarative library, instead of writing "how" the reconciler behaves, you instead describe "what" it's behavior should be like. Of course, that does not mean that the behavior has to be programmed, but instead that the declarative reconciler is built on a set of [declarative options](v2/options.go) that can be applied and result in changes to the reconciliation behavior of the actual underlying [reconciler](v2/reconciler.go).

The declarative reconciliation is strongly inspired by the [kubebuilder declarative pattern](https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern), however it brings it's own spin to it as it contains it's own form of applying and tracking changes after using a `rendering` engine, with different implementations:

- [HELM](v2/renderer_helm.go) is an optimized and specced-down version of the Helm Templating Engine to use with Charts
- [kustomize](v2/renderer_kustomize.go) is an embedded [krusty](https://pkg.go.dev/sigs.k8s.io/kustomize/api/krusty) kustomize implementation
- [raw](v2/renderer_raw.go) is an easy to use raw renderer that just passes through manifests
