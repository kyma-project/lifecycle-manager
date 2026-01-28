package skrwebhook

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal"
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

	ErrWatcherDirNotExist = errors.New("failed to locate watcher resource manifest folder")
)

const DefaultResourcesPath = "./skr-webhook"

type CertificateRepository interface {
	Create(ctx context.Context, name, commonName string, dnsNames []string) error
	Delete(ctx context.Context, name string) error
	Exists(ctx context.Context, certName string) (bool, error)
	Renew(ctx context.Context, name string) error
	GetRenewalTime(ctx context.Context, name string) (time.Time, error)
	GetValidity(ctx context.Context, name string) (time.Time, time.Time, error)
}

func ComposeSkrWebhookManager(kcpClient client.Client,
	skrContextProvider remote.SkrContextProvider,
	repository gateway.IstioGatewayRepository,
	certificateRepository CertificateRepository,
	flagVar *flags.FlagVar,
	watcherResourcesPath string,
) (*watcher.SkrWebhookManifestManager, error) {
	if watcherResourcesPath == "" {
		watcherResourcesPath = DefaultResourcesPath
	}
	dirInfo, err := os.Stat(watcherResourcesPath)
	if err != nil || !dirInfo.IsDir() {
		return nil, ErrWatcherDirNotExist
	}

	skrCertService := setupSKRCertService(kcpClient, certificateRepository, flagVar)

	gatewayService := gateway.NewService(flagVar.IstioGatewayName,
		flagVar.IstioGatewayNamespace,
		flagVar.ListenerPortOverwrite,
		repository,
	)

	resolvedKcpAddr, err := gatewayService.ResolveKcpAddr()
	if err != nil {
		return nil, err
	}

	if resolvedKcpAddr == nil {
		return nil, errUnresolvedKcpAddress
	}

	watcherMetrics := metrics.NewWatcherMetrics()

	resourceConfigurator := skrwebhookresources.NewResourceConfigurator(
		flagVar.RemoteSyncNamespace,
		flagVar.GetWatcherImage(),
		flagVar.WatcherResourceLimitsCPU,
		flagVar.WatcherResourceLimitsMemory,
		*resolvedKcpAddr,
		flagVar.SkrImagePullSecret,
	)

	chartReaderService := chartreader.NewService(watcherResourcesPath)

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

//nolint:ireturn // chosen implementation shall be abstracted
func ComposeCertificateRepository(kcpClient client.Client,
	flagVar *flags.FlagVar,
) (CertificateRepository, error) {
	certificateConfig := config.CertificateValues{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
		Namespace:   flagVar.IstioNamespace,
	}

	var certRepoImpl CertificateRepository
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

	return certRepoImpl, nil
}

func setupSKRCertService(kcpClient client.Client,
	certificateRepository CertificateRepository,
	flagVar *flags.FlagVar,
) *certificate.Service {
	certServiceConfig := certificate.Config{
		SkrServiceName:     skrwebhookresources.SkrResourceName,
		SkrNamespace:       flagVar.RemoteSyncNamespace,
		AdditionalDNSNames: strings.Split(flagVar.AdditionalDNSNames, ","),
		GatewaySecretName:  shared.GatewaySecretName,
		RenewBuffer:        flagVar.SelfSignedCertRenewBuffer,
	}

	secretRepository := secretrepo.NewRepository(kcpClient, flagVar.IstioNamespace)

	renewalService := renewal.NewService(certificateRepository,
		secretRepository,
		shared.GatewaySecretName,
	)

	return certificate.NewService(renewalService,
		certificateRepository,
		secretRepository,
		certServiceConfig,
	)
}
