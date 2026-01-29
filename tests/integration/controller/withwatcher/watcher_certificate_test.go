package withwatcher_test

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ErrPrivateKeyNotMatching = errors.New("private Key for the TLS secret doesn't match")

var _ = Describe("Watcher Certificate Configuration in remote sync mode", Ordered, func() {
	kyma := NewTestKyma("kyma-remote-sync-cert")

	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)
	issuer := NewTestIssuer(shared.IstioNamespace)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	tlsSecret := createWatcherSecret(kymaObjKey)
	skrTLSSecretObjKey := client.ObjectKey{
		Name:      skrwebhookresources.SkrTLSName,
		Namespace: flags.DefaultRemoteSyncNamespace,
	}
	gatewaySecret := createGatewaySecret()

	registerDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma, tlsSecret, issuer, gatewaySecret)
	var skrClient client.Client
	var err error
	BeforeAll(func() {
		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())
	})

	It("remote kyma created on SKR", func() {
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, shared.DefaultRemoteKymaName, flags.DefaultRemoteSyncNamespace).
			Should(Succeed())
	})

	It("mock cert-manager writing cert status", func() {
		Eventually(AddValidityToCertificateStatus, Timeout, Interval).
			WithContext(ctx).
			WithArguments(ctx,
				kcpClient,
				client.ObjectKeyFromObject(tlsSecret), // cert and secret have the same name and namespace
				time.Now(),
				time.Now().Add(24*time.Hour),
			).
			Should(Succeed())
	})

	It("kyma reconciliation creates Certificate CR on KCP", func() {
		Eventually(certificateExists, Timeout, Interval).
			WithArguments(kcpClient, kyma.Name).
			Should(Succeed())

		By("deleting the Certificate CR on KCP")
		certificateCR, err := getCertificate(kcpClient, kyma.Name)
		Expect(err).ToNot(HaveOccurred())
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, certificateCR).Should(Succeed())

		By("Certificate CR recreated on KCP")
		Eventually(certificateExists, Timeout, Interval).
			WithArguments(kcpClient, kyma.Name).
			Should(Succeed())
	})

	It("mock cert-manager writing cert status", func() {
		Eventually(AddValidityToCertificateStatus, Timeout, Interval).
			WithContext(ctx).
			WithArguments(ctx,
				kcpClient,
				client.ObjectKeyFromObject(tlsSecret), // cert and secret have the same name and namespace
				time.Now(),
				time.Now().Add(24*time.Hour),
			).
			Should(Succeed())
	})

	It("kyma reconciliation creates Certificate Secret on SKR", func() {
		Eventually(secretExists, Timeout, Interval).
			WithArguments(skrClient, skrTLSSecretObjKey).
			Should(Succeed())

		By("deleting the Certificate Secret on SKR")
		secret, err := getSecret(skrClient, skrTLSSecretObjKey)
		Expect(err).ToNot(HaveOccurred())
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, secret).Should(Succeed())

		By("recreated Certificate Secret on SKR")
		Eventually(secretExists, Timeout, Interval).
			WithArguments(skrClient, skrTLSSecretObjKey).
			Should(Succeed())
	})

	It("kyma reconciliation updates existing TLS Secret on SKR", func() {
		newKey := "new-pk"

		By("changing the TLS secret on KCP")
		tlsSecret.Data[apicorev1.TLSPrivateKeyKey] = []byte(newKey)
		Expect(kcpClient.Update(ctx, tlsSecret)).To(Succeed())

		By("updates the TLS secret on SKR")
		Eventually(matchTLSSecretPrivateKey, Timeout, Interval).
			WithArguments(skrClient, skrTLSSecretObjKey, []byte(newKey)).
			Should(Succeed())
	})

	AfterAll(func() {
		Expect(kcpClient.Delete(ctx, kyma)).To(Succeed())
	})
})

func getCertificate(clnt client.Client, kymaName string) (*certmanagerv1.Certificate, error) {
	certificateCR := &certmanagerv1.Certificate{}
	err := clnt.Get(ctx,
		client.ObjectKey{Name: kymaName + "-webhook-tls", Namespace: shared.IstioNamespace},
		certificateCR)
	return certificateCR, err
}

func getSecret(clnt client.Client, objKey client.ObjectKey) (*apicorev1.Secret, error) {
	secretCR := &apicorev1.Secret{}
	err := clnt.Get(ctx, objKey, secretCR)
	return secretCR, err
}

func certificateExists(clnt client.Client, kymaName string) error {
	_, err := getCertificate(clnt, kymaName)
	if util.IsNotFound(err) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return nil
}

func secretExists(clnt client.Client, secretObjKey client.ObjectKey) error {
	_, err := getSecret(clnt, secretObjKey)
	if util.IsNotFound(err) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return nil
}

func matchTLSSecretPrivateKey(clnt client.Client, secretObjKey client.ObjectKey, privateKey []byte) error {
	secretCR, err := getSecret(clnt, secretObjKey)
	if err != nil {
		return err
	}
	if !bytes.Equal(secretCR.Data[apicorev1.TLSPrivateKeyKey], privateKey) {
		return ErrPrivateKeyNotMatching
	}
	return nil
}
