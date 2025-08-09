package shared

const (
	CACertificateName        = "klm-watcher-serving"
	IstioNamespace           = "istio-system"
	GatewaySecretName        = "klm-istio-gateway" //nolint:gosec // It is just a name
	LastModifiedAtAnnotation = "lastModifiedAt"
)
