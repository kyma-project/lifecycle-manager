package composition

import (
	"os"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	certrepo "github.com/kyma-project/lifecycle-manager/internal/repository/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/repository/certificate/gardener"
)

func getCertManagerClient(kcp client.Client,
	flagVar *flags.FlagVar,
	config certrepo.CertificateConfig,
	_ logr.Logger,
) *certmanager.Certificate {
	return certmanager.NewCertificate(kcp,
		flagVar.SelfSignedCertificateIssuerName,
		config,
	)
}

func getGardenerCertificateManagementClient(kcp client.Client,
	flagVar *flags.FlagVar,
	config certrepo.CertificateConfig,
	setupLog logr.Logger,
) *gardener.Certificate {
	certClient, err := gardener.NewCertificate(kcp,
		flagVar.SelfSignedCertificateIssuerName,
		flagVar.SelfSignedCertIssuerNamespace,
		config,
	)
	if err != nil {
		setupLog.Error(err, "unable to initialize Gardener certificate management")
		os.Exit(bootstrapFailedExitCode)
	}

	return certClient
}
