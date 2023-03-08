# Internal Manifest Library

This package contains internal reference coding that is used for translating and implementing the Manifest Reconciler parts that are not generically solved by the [declarative library](../../pkg/declarative/README.md).

It contains various aspects to:
1. Translate the [Manifest](../../api/v1beta1/manifest_types.go) into a [declarative specification](../../pkg/declarative/v2/spec.go) in a custom [specification resolver](v1beta1/spec_resolver.go)
2. Lookup the correct Client for Single or Dual Cluster Mode based on the [SKR Client Lookup](v1beta1/skr_client_lookup.go) and a [ClusterClient](v1beta1/client.go) to read Secret Data as if they were a kubeconfig
3. Lookup the correct Keychain to access image layers contained in private registries through a [Keychain](v1beta1/keychain.go)
4. Interact with Default Data supplied to the `Manifest` for initializing a module with [Pre/Post-Hooks](v1beta1/custom_resource.go)
5. Check for Module Readiness not only by looking at the resources supplied in the chart, but also to introspect the already mentioned CustomResource using a [custom ReadyCheck](v1beta1/ready_check.go)
