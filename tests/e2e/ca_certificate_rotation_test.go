package e2e_test

import (
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("CA Certificate Rotation", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	var caCertificate *certmanagerv1.Certificate
	caCertName := "klm-watcher-serving-cert"

	Context("Given KCP Cluster and rotated CA certificate", func() {
		kcpNamespacedSecretName := k8stypes.NamespacedName{
			Name:      fmt.Sprintf("%s-webhook-tls", kyma.Name),
			Namespace: "istio-system",
		}

		skrNamespacedSecretName := k8stypes.NamespacedName{
			Name:      watcher.SkrTLSName,
			Namespace: remoteNamespace,
		}
		It("Then KCP TLS Certificate is removed", func() {
			timeNow := &apimetav1.Time{Time: time.Now()}
			expectedLogMessage := "CA Certificate was rotated, removing certificate"
			// The timeout used is 4 minutes bec the certificate gets rotated every 1 minute
			Eventually(CheckKLMLogs, 4*time.Minute).
				WithContext(ctx).
				WithArguments(expectedLogMessage, controlPlaneRESTConfig, runtimeRESTConfig,
					controlPlaneClient, runtimeClient, timeNow).
				Should(Succeed())

			By("And new TLS Certificate is created")
			var err error
			namespacedCertName := k8stypes.NamespacedName{
				Name:      caCertName,
				Namespace: "istio-system",
			}
			caCertificate, err = GetCACertificate(ctx, namespacedCertName, controlPlaneClient)
			Expect(err).NotTo(HaveOccurred())
			Eventually(CertificateSecretIsCreatedAfter).
				WithContext(ctx).
				WithArguments(kcpNamespacedSecretName, controlPlaneClient, caCertificate.Status.NotBefore).
				Should(Succeed())

			By("And new TLS Certificate is synced to SKR Cluster")
			Eventually(CertificateSecretIsSyncedToSkrCluster).
				WithContext(ctx).
				WithArguments(kcpNamespacedSecretName, controlPlaneClient, skrNamespacedSecretName, runtimeClient).
				Should(Succeed())
		})
	})
})
