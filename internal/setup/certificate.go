package setup

import (
	"os"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/gardener"
)

func setupCertManagerClient(kcpClient client.Client,
	flagVar *flags.FlagVar,
	config certificate.CertificateConfig,
	_ logr.Logger,
) *certmanager.CertificateClient {
	return certmanager.NewCertificateClient(kcpClient,
		flagVar.SelfSignedCertificateIssuerName,
		config,
	)
}

func setupGardenerCertificateManagementClient(kcpClient client.Client,
	flagVar *flags.FlagVar,
	config certificate.CertificateConfig,
	setupLog logr.Logger,
) *gardener.CertificateClient {
	certClient, err := gardener.NewCertificateClient(kcpClient,
		flagVar.SelfSignedCertificateIssuerName,
		flagVar.IstioNamespace,
		config,
	)
	if err != nil {
		setupLog.Error(err, "unable to initialize Gardener certificate management")
		os.Exit(bootstrapFailedExitCode)
	}

	return certClient
}
