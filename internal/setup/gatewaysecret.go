package setup

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common"
	gatewaysecretclient "github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/client"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/gardener"
)

func SetupCertInterface(kcpClient client.Client, flagVar *flags.FlagVar) (gatewaysecretclient.CertificateInterface, //nolint:ireturn // we select the interface here on purpose
	error,
) {
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
		return nil, common.ErrUnsupportedCertificateManagementSystem
	}
}
