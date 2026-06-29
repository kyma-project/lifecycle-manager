package spec

// Spec describes the resolved location and identity of a Manifest's installation
// layer. It is produced by Resolver.GetSpec and consumed by the manifest parser
// (to look up the manifest YAML on disk) and by the manifest controller (to
// detect OCI ref changes via the synced-OCI-ref annotation).
type Spec struct {
	ManifestName string
	Path         string
	OCIRef       string
}
