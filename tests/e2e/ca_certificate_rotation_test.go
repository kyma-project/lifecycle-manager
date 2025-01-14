package e2e_test

import (
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("CA Certificate Rotation", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	var caCertificate *certmanagerv1.Certificate
	caCertName := "klm-watcher-serving"

	Context("Given KCP Cluster and rotated CA certificate", func() {
		kcpSecretName := types.NamespacedName{
			Name:      kyma.Name + "-webhook-tls",
			Namespace: IstioNamespace,
		}
		skrSecretName := types.NamespacedName{
			Name:      watcher.SkrTLSName,
			Namespace: RemoteNamespace,
		}
		It("Then KCP TLS Certificate is removed", func() {
			var err error
			namespacedSecretName := types.NamespacedName{
				Name:      watcher.ResolveTLSCertName(kyma.Name),
				Namespace: IstioNamespace,
			}
			tlsSecret, err := GetTLSSecret(ctx, namespacedSecretName, kcpClient)
			Expect(err).NotTo(HaveOccurred())

			// The timeout used is 4 minutes bec the certificate gets rotated every 1 minute
			Eventually(TLSSecretRotated, 4*time.Minute).
				WithContext(ctx).
				WithArguments(tlsSecret.CreationTimestamp.Time, namespacedSecretName, kcpClient).
				Should(Succeed())

			By("And new TLS Certificate is created")
			namespacedCertName := types.NamespacedName{
				Name:      caCertName,
				Namespace: IstioNamespace,
			}
			caCertificate, err = GetCACertificate(ctx, namespacedCertName, kcpClient)
			Expect(err).NotTo(HaveOccurred())
			Eventually(CertificateSecretIsCreatedAfter).
				WithContext(ctx).
				WithArguments(kcpSecretName, kcpClient, caCertificate.Status.NotBefore).
				Should(Succeed())

			By("And new TLS Certificate is synced to SKR Cluster")
			Eventually(CertificateSecretIsSyncedToSkrCluster).
				WithContext(ctx).
				WithArguments(kcpSecretName, kcpClient, skrSecretName, skrClient).
				Should(Succeed())
		})
	})
})
