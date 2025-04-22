package setup

import (
	"os"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common"
	gatewaysecretclient "github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/client"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
)

//nolint:ireturn // chosen implementation shall be abstracted
func SetupCertInterface(kcpClient client.Client,
	flagVar *flags.FlagVar,
	setupLog logr.Logger,
) gatewaysecretclient.CertificateInterface {
	certificateConfig := certificate.CertificateConfig{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
	}

	setupFunc, ok := map[string]func() gatewaysecretclient.CertificateInterface{
		certmanagerv1.SchemeGroupVersion.String(): func() gatewaysecretclient.CertificateInterface {
			return setupCertManagerClient(kcpClient, flagVar, certificateConfig, setupLog)
		},
		gcertv1alpha1.SchemeGroupVersion.String(): func() gatewaysecretclient.CertificateInterface {
			return setupGardenerCertificateManagementClient(kcpClient, flagVar, certificateConfig, setupLog)
		},
	}[flagVar.CertificateManagement]

	if !ok {
		setupLog.Error(common.ErrUnsupportedCertificateManagementSystem,
			"unable to initialize gatwaysecret certificate interface")
		os.Exit(bootstrapFailedExitCode)
	}

	return setupFunc()
}
