package skrwebhook

import (
	"errors"
	"strings"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/service/gateway"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/gardener"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/secret"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/pkg/watcher/skr_webhook_resources"
)

var (
	errUnresolvedKcpAddress   = errors.New("failed to resolve KCP address, please check the gateway configuration")
	errCertClientNotSupported = errors.New("certificate client not supported, please check the certificate management configuration")
)

func ComposeSkrWebhookManager(kcpClient client.Client, skrContextFactory remote.SkrContextProvider,
	repository gateway.IstioGatewayRepository, flagVar *flags.FlagVar,
) (*watcher.SkrWebhookManifestManager, error) {
	certManager, err := setupCertManager(kcpClient, flagVar)
	if err != nil {
		return nil, err
	}

	gatewayService := gateway.NewService(flagVar.IstioGatewayName, flagVar.IstioGatewayNamespace,
		flagVar.ListenerPortOverwrite, repository)

	resolvedKcpAddr, err := gatewayService.ResolveKcpAddr()
	if err != nil {
		return nil, err
	}

	if resolvedKcpAddr == nil {
		return nil, errUnresolvedKcpAddress
	}

	watcherMetrics := metrics.NewWatcherMetrics()

	config := watcher.SkrWebhookManagerConfig{
		SkrWatcherPath:         flagVar.WatcherResourcesPath,
		SkrWatcherImage:        flagVar.GetWatcherImage(),
		SkrWebhookCPULimits:    flagVar.WatcherResourceLimitsCPU,
		SkrWebhookMemoryLimits: flagVar.WatcherResourceLimitsMemory,
		RemoteSyncNamespace:    flagVar.RemoteSyncNamespace,
	}

	return watcher.NewSKRWebhookManifestManager(
		kcpClient,
		skrContextFactory,
		config,
		*resolvedKcpAddr,
		certManager,
		watcherMetrics)
}

func setupCertManager(kcpClient client.Client, flagVar *flags.FlagVar) (*certificate.CertificateManager, error) {
	certClient, err := setupCertClient(kcpClient, flagVar)
	if err != nil {
		return nil, err
	}
	secretClient := secret.NewCertificateSecretClient(kcpClient)

	config := certificate.CertificateManagerConfig{
		SkrServiceName:               skrwebhookresources.SkrResourceName,
		SkrNamespace:                 flagVar.RemoteSyncNamespace,
		CertificateNamespace:         flagVar.IstioNamespace,
		AdditionalDNSNames:           strings.Split(flagVar.AdditionalDNSNames, ","),
		GatewaySecretName:            shared.GatewaySecretName,
		RenewBuffer:                  flagVar.SelfSignedCertRenewBuffer,
		SkrCertificateNamingTemplate: flagVar.SelfSignedCertificateNamingTemplate,
	}

	return certificate.NewCertificateManager(
		certClient,
		secretClient,
		config,
	), nil
}

func setupCertClient(kcpClient client.Client, flagVar *flags.FlagVar) (certificate.CertificateClient, error) {
	certificateConfig := certificate.CertificateConfig{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
	}

	switch flagVar.CertificateManagement {
	case certmanagerv1.SchemeGroupVersion.String():
		return certmanager.NewCertificateClient(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			certificateConfig,
		), nil
	case gcertv1alpha1.SchemeGroupVersion.String():
		return gardener.NewCertificateClient(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			flagVar.SelfSignedCertIssuerNamespace,
			certificateConfig,
		)
	default:
		return nil, errCertClientNotSupported
	}
}
