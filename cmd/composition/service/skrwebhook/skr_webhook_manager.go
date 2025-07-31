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
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/config"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/gcm"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/secret"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/chartreader"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/gateway"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

var (
	errUnresolvedKcpAddress                = errors.New("failed to resolve KCP address, please check the gateway configuration")
	errCertReposImplementationNotSupported = errors.New("certificate client not supported, please check the certificate management configuration")
)

func ComposeSkrWebhookManager(kcpClient client.Client,
	skrContextProvider remote.SkrContextProvider,
	repository gateway.IstioGatewayRepository,
	flagVar *flags.FlagVar,
) (*watcher.SkrWebhookManifestManager, error) {
	skrCertService, err := setupSKRCertService(kcpClient, flagVar)
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

	resourceConfigurator := skrwebhookresources.NewResourceConfigurator(
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
		skrCertService,
		resourceConfigurator,
		watcherMetrics)
}

func setupSKRCertService(kcpClient client.Client, flagVar *flags.FlagVar) (*certificate.SKRCertService, error) {
	certificateConfig := config.CertificateValues{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
		Namespace:   flagVar.IstioNamespace,
	}

	var certRepoImpl certificate.CertRepository
	var err error
	switch flagVar.CertificateManagement {
	case certmanagerv1.SchemeGroupVersion.String():
		certRepoImpl, err = certmanager.NewCertificateRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			certificateConfig)
	case gcertv1alpha1.SchemeGroupVersion.String():
		certRepoImpl, err = gcm.NewCertificateRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			flagVar.SelfSignedCertIssuerNamespace,
			certificateConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errCertReposImplementationNotSupported
	}

	config := certificate.Config{
		SkrServiceName:     skrwebhookresources.SkrResourceName,
		SkrNamespace:       flagVar.RemoteSyncNamespace,
		AdditionalDNSNames: strings.Split(flagVar.AdditionalDNSNames, ","),
		GatewaySecretName:  shared.GatewaySecretName,
		RenewBuffer:        flagVar.SelfSignedCertRenewBuffer,
	}

	return certificate.NewSKRCertService(
		certRepoImpl,
		secret.NewCertificateSecretRepository(kcpClient, flagVar.IstioNamespace),
		config,
	), nil
}
