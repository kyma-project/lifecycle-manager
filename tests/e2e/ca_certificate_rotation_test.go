package e2e_test

import (
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
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
			Namespace: "istio-system",
		}
		skrSecretName := types.NamespacedName{
			Name:      watcher.SkrTLSName,
			Namespace: RemoteNamespace,
		}
		It("Then KCP TLS Certificate is removed", func() {
			timeNow := &apimetav1.Time{Time: time.Now()}
			expectedLogMessage := "CA Certificate was rotated, removing certificate"
			// The timeout used is 4 minutes bec the certificate gets rotated every 1 minute
			Eventually(CheckPodLogs, 4*time.Minute).
				WithContext(ctx).
				WithArguments(ControlPlaneNamespace, KLMPodPrefix, KLMPodContainer, expectedLogMessage, kcpRESTConfig,
					kcpClient, timeNow).
				Should(Succeed())

			By("And new TLS Certificate is created")
			var err error
			namespacedCertName := types.NamespacedName{
				Name:      caCertName,
				Namespace: "istio-system",
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
