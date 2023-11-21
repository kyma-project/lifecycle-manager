package withwatcher_test

import (
	"bytes"
	"errors"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var ErrPrivateKeyNotMatching = errors.New("private Key for the TLS secret doesn't match")

func getCertificate(clnt client.Client, kymaName string) (*certmanagerv1.Certificate, error) {
	certificateCR := &certmanagerv1.Certificate{}
	err := clnt.Get(suiteCtx,
		client.ObjectKey{Name: watcher.ResolveTLSCertName(kymaName), Namespace: istioSystemNs},
		certificateCR)
	return certificateCR, err
}

func getSecret(clnt client.Client, objKey client.ObjectKey) (*apicorev1.Secret, error) {
	secretCR := &apicorev1.Secret{}
	err := clnt.Get(suiteCtx, objKey, secretCR)
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

var _ = Describe("Watcher Certificate Configuration in remote sync mode", Ordered, func() {
	caCertificate := &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "klm-watcher-serving-cert",
			Namespace: istioSystemNs,
		},
		Spec: certmanagerv1.CertificateSpec{
			DNSNames:   []string{"listener.kyma.cloud.sap"},
			IsCA:       true,
			CommonName: "klm-watcher-selfsigned-ca",
			SecretName: "klm-watcher-root-secret",
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: map[string]string{
					"operator.kyma-project.io/managed-by": "lifecycle-manager",
				},
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.PrivateKeyAlgorithm("RSA"),
			},
		},
	}

	kyma := NewTestKyma("kyma-remote-sync")

	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)
	issuer := NewTestIssuer(istioSystemNs)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	tlsSecret := createTLSSecret(kymaObjKey)
	skrTLSSecretObjKey := client.ObjectKey{Name: watcher.SkrTLSName, Namespace: controller.DefaultRemoteSyncNamespace}

	registerDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma, tlsSecret, issuer, caCertificate)
	It("remote kyma created on SKR", func() {
		Eventually(KymaExists, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(runtimeClient, v1beta2.DefaultRemoteKymaName, controller.DefaultRemoteSyncNamespace).
			Should(Succeed())
	})

	It("kyma reconciliation creates Certificate CR on KCP", func() {
		Eventually(certificateExists, Timeout, Interval).
			WithArguments(controlPlaneClient, kyma.Name).
			Should(Succeed())

		By("deleting the Certificate CR on KCP")
		certificateCR, err := getCertificate(controlPlaneClient, kyma.Name)
		Expect(err).ToNot(HaveOccurred())
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(runtimeClient, certificateCR).Should(Succeed())

		By("Certificate CR recreated on KCP")
		Eventually(certificateExists, Timeout, Interval).
			WithArguments(controlPlaneClient, kyma.Name).
			Should(Succeed())
	})

	It("kyma reconciliation creates Certificate Secret on SKR", func() {
		Eventually(secretExists, Timeout, Interval).
			WithArguments(runtimeClient, skrTLSSecretObjKey).
			Should(Succeed())

		By("deleting the Certificate Secret on SKR")
		secret, err := getSecret(runtimeClient, skrTLSSecretObjKey)
		Expect(err).ToNot(HaveOccurred())
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(runtimeClient, secret).Should(Succeed())

		By("recreated Certificate Secret on SKR")
		Eventually(secretExists, Timeout, Interval).
			WithArguments(runtimeClient, skrTLSSecretObjKey).
			Should(Succeed())
	})

	It("kyma reconciliation updates existing TLS Secret on SKR", func() {
		newKey := "new-pk"

		By("changing the TLS secret on KCP")
		tlsSecret.Data[apicorev1.TLSPrivateKeyKey] = []byte(newKey)
		Expect(controlPlaneClient.Update(suiteCtx, tlsSecret)).To(Succeed())

		By("updates the TLS secret on SKR")
		Eventually(matchTLSSecretPrivateKey, Timeout, Interval).
			WithArguments(runtimeClient, skrTLSSecretObjKey, []byte(newKey)).
			Should(Succeed())
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(suiteCtx, kyma)).To(Succeed())
	})
})
