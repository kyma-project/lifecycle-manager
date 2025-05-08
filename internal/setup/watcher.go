package setup

import (
	"os"
	"strings"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	certrepo "github.com/kyma-project/lifecycle-manager/internal/repository/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	certsvc "github.com/kyma-project/lifecycle-manager/internal/service/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

const bootstrapFailedExitCode = 1

func SetupSkrWebhookManager(mgr ctrl.Manager,
	skrContextFactory remote.SkrContextProvider,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) *watcher.SkrWebhookManifestManager {
	kcpClient := mgr.GetClient()

	certManager := setupCertManager(kcpClient, flagVar, setupLog)

	resolvedKcpAddr := getResolvedKcpAddress(mgr, flagVar, setupLog)

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

func getResolvedKcpAddress(mgr ctrl.Manager,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) string {
	gatewayConfig := watcher.GatewayConfig{
		IstioGatewayName:          flagVar.IstioGatewayName,
		IstioGatewayNamespace:     flagVar.IstioGatewayNamespace,
		LocalGatewayPortOverwrite: flagVar.ListenerPortOverwrite,
	}

	resolvedKcpAddr, err := gatewayConfig.ResolveKcpAddr(mgr)
	if err != nil {
		setupLog.Error(err, "failed to resolve KCP address")
		os.Exit(bootstrapFailedExitCode)
	}

	return resolvedKcpAddr
}

func setupCertManager(kcpClient client.Client,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) *certsvc.CertificateManager {
	certClient := setupCertClient(kcpClient, flagVar, setupLog)

	secretClient := secret.NewCertificateSecretClient(kcpClient)

	config := certsvc.CertificateManagerConfig{
		SkrServiceName:               watcher.SkrResourceName,
		SkrNamespace:                 flagVar.RemoteSyncNamespace,
		CertificateNamespace:         flagVar.IstioNamespace,
		AdditionalDNSNames:           strings.Split(flagVar.AdditionalDNSNames, ","),
		GatewaySecretName:            shared.GatewaySecretName,
		RenewBuffer:                  flagVar.SelfSignedCertRenewBuffer,
		SkrCertificateNamingTemplate: flagVar.SelfSignedCertificateNamingTemplate,
	}

	return certsvc.NewCertificateManager(
		certClient,
		secretClient,
		config,
	)
}

//nolint:ireturn // chosen implementation shall be abstracted
func setupCertClient(kcpClient client.Client,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) certsvc.Certificate {
	certificateConfig := certrepo.CertificateConfig{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
	}

	setupFunc, ok := map[string]func() certsvc.Certificate{
		certmanagerv1.SchemeGroupVersion.String(): func() certsvc.Certificate {
			return setupCertManagerClient(kcpClient, flagVar, certificateConfig, setupLog)
		},
		gcertv1alpha1.SchemeGroupVersion.String(): func() certsvc.Certificate {
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
