package setup

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common"
	gatewaysecretclient "github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/client"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
)

//nolint:ireturn // chosen implementation shall be abstracted
func SetupCertInterface(kcpClient client.Client, flagVar *flags.FlagVar) (gatewaysecretclient.CertificateInterface,
	error,
) {
	certificateConfig := config.CertificateValues{
		Duration:    flagVar.SelfSignedCertDuration,
		RenewBefore: flagVar.SelfSignedCertRenewBefore,
		KeySize:     flagVar.SelfSignedCertKeySize,
		Namespace:   shared.IstioNamespace,
	}
	switch flagVar.CertificateManagement {
	case certmanagerv1.SchemeGroupVersion.String():
		return certmanagercertificate.NewRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			certificateConfig,
		)
	case gcertv1alpha1.SchemeGroupVersion.String():
		return gcmcertificate.NewRepository(kcpClient,
			flagVar.SelfSignedCertificateIssuerName,
			flagVar.SelfSignedCertIssuerNamespace,
			certificateConfig,
		)
	default:
		return nil, common.ErrUnsupportedCertificateManagementSystem
	}
}
