# Internal Manifest Library

This package contains internal reference coding that is used for translating and implementing the Manifest Reconciler parts that are not generically solved by the [declarative library](../declarative/).

It contains various aspects to:

1. Translate the [Manifest CR](../../api/v1beta2/manifest_types.go) into a [declarative specification](../declarative/v2/spec.go) in a custom [specification resolver](spec_resolver.go). Use [the layer parsing methods](img/parse.go) to download and resolve OCI image layers from the Manifest to make them accessible to the declarative library.
2. Look up the correct Client for Single or Dual Cluster Mode based on the [SKR Client Lookup](skr_client_lookup.go) and [ClusterClient](client.go) to read Secret Data as if they were a kubeconfig.
3. Look up the correct Keychain to access image layers contained in private registries through a [Keychain](../../pkg/ocmextensions/cred.go).
5. Check for Module Readiness based on the applied resources. For more information, see [custom StateCheck](statecheck/state_check.go).
