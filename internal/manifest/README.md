# Internal Manifest Library

This package contains internal reference coding that is used for translating and implementing the Manifest Reconciler parts that are not generically solved by the [declarative library](/internal/declarative/README.md).

It contains various aspects to:
1. Translate the [Manifest](/api/v1beta1/manifest_types.go) into a [declarative specification](/internal/declarative/v2/spec.go) in a custom [specification resolver](spec_resolver.go). Use [the layer parsing methods](parse.go) to download and resolve OCI image layers from the Manifest to make them accessible to the declarative library.
2. Lookup the correct Client for Single or Dual Cluster Mode based on the [SKR Client Lookup](skr_client_lookup.go) and a [ClusterClient](client.go) to read Secret Data as if they were a kubeconfig
3. Lookup the correct Keychain to access image layers contained in private registries through a [Keychain](/pkg/ocmextensions/cred.go)
4. Interact with Default Data supplied to the `Manifest` for initializing a module with [Pre/Post-Hooks](custom_resource.go)
5. Check for Module Readiness not only by looking at the resources supplied in the chart, but also to introspect the already mentioned CustomResource using a [custom ReadyCheck](ready_check.go)
