package e2e_test

import (
	"fmt"
	"time"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	var caCertificate *v1.Certificate
	caCertName := "klm-watcher-serving-cert"

	Context("Given Kyma deployed in KCP and CA certificate is rotated", func() {
		It("Then Kyma certificate is removed", func() {
			timeNow := &apimetav1.Time{Time: time.Now()}
			expectedLogMessage := "CA Certificate was rotated, removing certificate"
			// The timeout used is 4 minutes bec the certificate gets rotated every 1 minute
			Eventually(CheckKLMLogs, 4*time.Minute).
				WithContext(ctx).
				WithArguments(remoteNamespace, expectedLogMessage, controlPlaneRESTConfig, runtimeRESTConfig,
					controlPlaneClient, runtimeClient, timeNow).
				Should(Succeed())
		})

		It("And new certificate is created", func() {
			var err error
			caCertificate, err = GetCACertificate(ctx, caCertName, "istio-system",
				controlPlaneClient)
			Expect(err).NotTo(HaveOccurred())

			Eventually(CertificateSecretIsCreatedAfter).
				WithContext(ctx).
				WithArguments(fmt.Sprintf("%s-webhook-tls", kyma.Name), "istio-system", controlPlaneClient,
					caCertificate.Status.NotBefore).
				Should(Succeed())
		})

		It("And new certificate is synced to SKR cluster", func() {
			Eventually(CertificateSecretIsSyncedToSkrCluster).
				WithContext(ctx).
				WithArguments(fmt.Sprintf("%s-webhook-tls", kyma.Name), "istio-system", controlPlaneClient,
					watcher.SkrTLSName, remoteNamespace, runtimeClient).
				Should(Succeed())
		})
	})
})
