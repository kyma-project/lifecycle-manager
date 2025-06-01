package setup

import (
	"os"
	"strings"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/go-logr/logr"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/secret"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/pkg/watcher/skr_webhook_resources"
)

const bootstrapFailedExitCode = 1

func SetupSkrWebhookManager(mgr ctrl.Manager,
	skrContextFactory remote.SkrContextProvider,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) *watcher.SkrWebhookManifestManager {
	kcpClient := mgr.GetClient()

	certManager := setupCertManager(kcpClient, flagVar, setupLog)

	resolvedKcpAddr := getResolvedKcpAddress(mgr.GetConfig(), mgr.GetScheme(), flagVar, setupLog)

	watcherMetrics := metrics.NewWatcherMetrics()

	config := watcher.SkrWebhookManagerConfig{
		SkrWatcherPath:         flagVar.WatcherResourcesPath,
		SkrWatcherImage:        flagVar.GetWatcherImage(),
		SkrWebhookCPULimits:    flagVar.WatcherResourceLimitsCPU,
		SkrWebhookMemoryLimits: flagVar.WatcherResourceLimitsMemory,
		RemoteSyncNamespace:    flagVar.RemoteSyncNamespace,
	}

	skrWebhookManifestManager, err := watcher.NewSKRWebhookManifestManager(
		kcpClient,
		skrContextFactory,
		config,
		resolvedKcpAddr,
		certManager,
		watcherMetrics)
	if err != nil {
		setupLog.Error(err, "failed to setup SKR webhook manager")
		os.Exit(bootstrapFailedExitCode)
	}

	return skrWebhookManifestManager
}

func getResolvedKcpAddress(config *rest.Config, scheme *machineryruntime.Scheme,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) skrwebhookresources.KCPAddr {
	gatewayConfig := watcher.GatewayConfig{
		IstioGatewayName:          flagVar.IstioGatewayName,
		IstioGatewayNamespace:     flagVar.IstioGatewayNamespace,
		LocalGatewayPortOverwrite: flagVar.ListenerPortOverwrite,
	}
	kcpClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "can't create kcpClient")
		os.Exit(bootstrapFailedExitCode)
	}
	resolvedKcpAddr, err := gatewayConfig.ResolveKcpAddr(kcpClient)
	if err != nil || resolvedKcpAddr == nil {
		setupLog.Error(err, "failed to resolve KCP address")
		os.Exit(bootstrapFailedExitCode)
	}

	return *resolvedKcpAddr
}

func setupCertManager(kcpClient client.Client,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) *certificate.CertificateManager {
	certClient := setupCertClient(kcpClient, flagVar, setupLog)

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
	)
}

func setupCertClient(kcpClient client.Client,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) certificate.CertificateClient {
	certificateConfig := certificate.CertificateConfig{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
	}

	setupFunc, ok := map[string]func() certificate.CertificateClient{
		certmanagerv1.SchemeGroupVersion.String(): func() certificate.CertificateClient {
			return setupCertManagerClient(kcpClient, flagVar, certificateConfig, setupLog)
		},
		gcertv1alpha1.SchemeGroupVersion.String(): func() certificate.CertificateClient {
			return setupGardenerCertificateManagementClient(kcpClient, flagVar, certificateConfig, setupLog)
		},
	}[flagVar.CertificateManagement]

	if !ok {
		setupLog.Error(common.ErrUnsupportedCertificateManagementSystem,
			"unable to initialize certificate managemer client")
		os.Exit(bootstrapFailedExitCode)
	}

	return setupFunc()
}
