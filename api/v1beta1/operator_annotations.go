package v1beta1

const (
	FQDN = OperatorPrefix + Separator + "fqdn"

	// OwnedByAnnotation defines the resource managing the resource. Differing from ManagedBy
	// in that it does not reference controllers. Used by the runtime-watcher to determine the
	// corresponding CR in KCP.
	OwnedByAnnotation                        = OperatorPrefix + Separator + "owned-by"
	OwnedByFormat                            = "%s/%s"
	SkrKymaCRDGenerationAnnotation           = OperatorPrefix + Separator + "skr-kyma-crd-generation"
	KcpKymaCRDGenerationAnnotation           = OperatorPrefix + Separator + "kcp-kyma-crd-generation"
	SkrModuleTemplateCRDGenerationAnnotation = OperatorPrefix + Separator + "skr-moduletemplate-crd-generation"
	KcpModuleTemplateCRDGenerationAnnotation = OperatorPrefix + Separator + "kcp-moduletemplate-crd-generation"
)
