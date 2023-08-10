package withwatcher_test

import (
	"bytes"
	"errors"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

func getSecret(clnt client.Client, objKey client.ObjectKey) (*corev1.Secret, error) {
	secretCR := &corev1.Secret{}
	err := clnt.Get(suiteCtx, objKey, secretCR)
	return secretCR, err
}

func certificateExists(clnt client.Client, kymaName string) error {
	_, err := getCertificate(clnt, kymaName)
	return err
}

func secretExists(clnt client.Client, secretObjKey client.ObjectKey) error {
	_, err := getSecret(clnt, secretObjKey)
	return err
}

func deleteCertificate(clnt client.Client, kymaName string) error {
	certificateCR, err := getCertificate(clnt, kymaName)
	if err != nil {
		return err
	}
	err = clnt.Delete(suiteCtx, certificateCR)
	return err
}

func deleteSecret(clnt client.Client, secretObjKey client.ObjectKey) error {
	secretCR, err := getSecret(clnt, secretObjKey)
	if err != nil {
		return err
	}
	err = runtimeClient.Delete(suiteCtx, secretCR)
	return err
}

func matchTLSSecretPrivateKey(clnt client.Client, secretObjKey client.ObjectKey, privateKey []byte) error {
	secretCR, err := getSecret(clnt, secretObjKey)
	if err != nil {
		return err
	}
	if !bytes.Equal(secretCR.Data[corev1.TLSPrivateKeyKey], privateKey) {
		return ErrPrivateKeyNotMatching
	}
	return nil
}

var _ = Describe("Watcher Certificate Configuration in remote sync mode", Ordered, func() {
	kyma := NewTestKyma("kyma-remote-sync")

	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)
	issuer := NewTestIssuer(istioSystemNs)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	tlsSecret := createTLSSecret(kymaObjKey)
	skrTLSSecretObjKey := client.ObjectKey{Name: watcher.SkrTLSName, Namespace: controllers.DefaultRemoteSyncNamespace}

	registerDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma, tlsSecret, issuer)

	It("kyma reconciliation creates Certificate CR on KCP", func() {
		Eventually(certificateExists, Timeout, Interval).
			WithArguments(controlPlaneClient, kyma.Name).
			Should(Succeed())

		By("deleting the Certificate CR on KCP")
		Expect(deleteCertificate(controlPlaneClient, kyma.Name)).To(Succeed())

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
		Expect(deleteSecret(runtimeClient, skrTLSSecretObjKey)).To(Succeed())

		By("recreated Certificate Secret on SKR")
		Eventually(secretExists, Timeout, Interval).
			WithArguments(runtimeClient, skrTLSSecretObjKey).
			Should(Succeed())
	})

	It("kyma reconciliation updates existing TLS Secret on SKR", func() {
		newKey := "new-pk"

		By("changing the TLS secret on KCP")
		tlsSecret.Data[corev1.TLSPrivateKeyKey] = []byte(newKey)
		Expect(controlPlaneClient.Update(suiteCtx, tlsSecret)).To(Succeed())

		By("updates the TLS secret on SKR")
		Eventually(matchTLSSecretPrivateKey, Timeout, Interval).
			WithArguments(runtimeClient, skrTLSSecretObjKey, []byte(newKey)).
			Should(Succeed())
	})
})
