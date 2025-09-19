package skrwebhook

import (
	"errors"
	"fmt"
	"strings"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	secretrepository "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/renewal"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	certmanagerrenewal "github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal/certmanager" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal/gcm"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/chartreader"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/gateway"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

var (
	errUnresolvedKcpAddress = errors.New(
		"failed to resolve KCP address, please check the gateway configuration",
	)
	errCertificateManagementNotSupported = errors.New(
		"certificate management not supported, please check the certificate management configuration",
	)
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

func setupSKRCertService(kcpClient client.Client, flagVar *flags.FlagVar) (*certificate.Service, error) {
	certificateConfig := config.CertificateValues{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
		Namespace:   flagVar.IstioNamespace,
	}

	var certRepoImpl certificate.CertificateRepository
	var err error
	switch flagVar.CertificateManagement {
	case certmanagerv1.SchemeGroupVersion.String():
		certRepoImpl, err = certmanagercertificate.NewRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			certificateConfig)
	case gcertv1alpha1.SchemeGroupVersion.String():
		certRepoImpl, err = gcmcertificate.NewRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			flagVar.SelfSignedCertIssuerNamespace,
			certificateConfig)
	default:
		return nil, errCertificateManagementNotSupported
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate repository: %w", err)
	}

	certServiceConfig := certificate.Config{
		SkrServiceName:     skrwebhookresources.SkrResourceName,
		SkrNamespace:       flagVar.RemoteSyncNamespace,
		AdditionalDNSNames: strings.Split(flagVar.AdditionalDNSNames, ","),
		GatewaySecretName:  shared.GatewaySecretName,
		RenewBuffer:        flagVar.SelfSignedCertRenewBuffer,
	}

	secretRepository := secretrepository.NewRepository(kcpClient, flagVar.IstioNamespace)

	var renewalService certificate.RenewalService
	switch flagVar.CertificateManagement {
	case certmanagerv1.SchemeGroupVersion.String():
		renewalService = certmanagerrenewal.NewService(secretRepository)
	case gcertv1alpha1.SchemeGroupVersion.String():
		renewalCertRepo := renewal.NewRepository(kcpClient, flagVar.IstioNamespace)
		renewalService = gcm.NewService(renewalCertRepo)
	default:
		return nil, errCertificateManagementNotSupported
	}

	return certificate.NewService(renewalService, certRepoImpl, secretRepository, certServiceConfig), nil
}
