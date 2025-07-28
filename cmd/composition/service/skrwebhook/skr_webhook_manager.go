package skrwebhook

import (
	"errors"
	"strings"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/config"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/gcm"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/secret"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/chartreader"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/gateway"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

var (
	errUnresolvedKcpAddress   = errors.New("failed to resolve KCP address, please check the gateway configuration")
	errCertClientNotSupported = errors.New("certificate client not supported, please check the certificate management configuration")
)

func ComposeSkrWebhookManager(kcpClient client.Client,
	skrContextProvider remote.SkrContextProvider,
	repository gateway.IstioGatewayRepository,
	flagVar *flags.FlagVar,
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

	resourceConfigurator := resources.NewResourceConfigurator(
		flagVar.RemoteSyncNamespace, flagVar.GetWatcherImage(),
		flagVar.WatcherResourceLimitsCPU,
		flagVar.WatcherResourceLimitsMemory, *resolvedKcpAddr)

	chartReaderService := chartreader.NewService(flagVar.WatcherResourcesPath)

	return watcher.NewSKRWebhookManifestManager(
		kcpClient,
		skrContextProvider,
		flagVar.RemoteSyncNamespace,
		*resolvedKcpAddr,
		chartReaderService,
		certManager,
		resourceConfigurator,
		watcherMetrics)
}

func setupCertManager(kcpClient client.Client, flagVar *flags.FlagVar) (*certificate.SKRCertService, error) {
	certClient, err := setupCertClient(kcpClient, flagVar)
	if err != nil {
		return nil, err
	}
	secretClient := secret.NewCertificateSecretRepository(kcpClient, flagVar.IstioNamespace)

	config := certificate.Config{
		SkrServiceName:     resources.SkrResourceName,
		SkrNamespace:       flagVar.RemoteSyncNamespace,
		AdditionalDNSNames: strings.Split(flagVar.AdditionalDNSNames, ","),
		GatewaySecretName:  shared.GatewaySecretName,
		RenewBuffer:        flagVar.SelfSignedCertRenewBuffer,
	}

	return certificate.NewSKRCertService(
		certClient,
		secretClient,
		config,
	), nil
}

func setupCertClient(kcpClient client.Client, flagVar *flags.FlagVar) (certificate.CertRepository, error) {
	certificateConfig := config.CertificateValues{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
	}

	switch flagVar.CertificateManagement {
	case certmanagerv1.SchemeGroupVersion.String():
		return certmanager.NewCertificateRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			flagVar.IstioNamespace,
			certificateConfig,
		), nil
	case gcertv1alpha1.SchemeGroupVersion.String():
		return gcm.NewCertificateRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			flagVar.SelfSignedCertIssuerNamespace,
			certificateConfig,
		)
	default:
		return nil, errCertClientNotSupported
	}
}
