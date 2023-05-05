package v1beta2

const (
	FQDN = OperatorPrefix + Separator + "fqdn"

	// OwnedByAnnotation defines the resource managing the resource. Differing from ManagedBy
	// in that it does not reference controllers. Used by the runtime-watcher to determine the
	// corresponding CR in KCP.
	OwnedByAnnotation                        = OperatorPrefix + Separator + "owned-by"
	OwnedByFormat                            = "%s/%s"
	SyncStrategyAnnotation                   = "sync-strategy"
	SkrKymaCRDGenerationAnnotation           = "skr-kyma-crd-generation"
	KcpKymaCRDGenerationAnnotation           = "kcp-kyma-crd-generation"
	SkrModuleTemplateCRDGenerationAnnotation = "skr-moduletemplate-crd-generation"
	KcpModuleTemplateCRDGenerationAnnotation = "kcp-moduletemplate-crd-generation"
)
