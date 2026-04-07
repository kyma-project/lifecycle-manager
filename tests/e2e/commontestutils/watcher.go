package commontestutils

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrSecretNotFound         = errors.New("secret does not exist")
	ErrCertificateNotFound    = errors.New("certificate does not exist")
	errNotSyncedSecret        = errors.New("secrets are not synced")
	errCreationTimeNotUpdated = errors.New("gateway secret has an old creation timestamp")
)

func CertificateSecretExists(ctx context.Context, secretName types.NamespacedName, k8sClient client.Client) error {
	certificateSecret := &apicorev1.Secret{}
	err := k8sClient.Get(ctx, secretName, certificateSecret)
	if util.IsNotFound(err) {
		return ErrSecretNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get certificate secret %w", err)
	}

	return nil
}

func CertificateExists(ctx context.Context, certificateName types.NamespacedName, k8sClient client.Client) error {
	certificate := &certmanagerv1.Certificate{}
	err := k8sClient.Get(ctx, certificateName, certificate)
	if util.IsNotFound(err) {
		return ErrCertificateNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get certificate %w", err)
	}

	return nil
}

func IstioGatewaySecretIsSyncedToRootCA(ctx context.Context,
	rootCASecretName types.NamespacedName, kcpClient client.Client,
) error {
	rootCASecret, err := fetchCertificateSecret(ctx, rootCASecretName, kcpClient)
	if err != nil {
		return fmt.Errorf("failed to fetch root CA secret: %w", err)
	}

	gatewaySecret, err := GetGatewaySecret(ctx, kcpClient)
	if err != nil {
		return fmt.Errorf("failed to fetch istio gateway secret: %w", err)
	}

	err = verifySecretsHaveSameData(rootCASecret, gatewaySecret)
	if err != nil {
		return err
	}

	return nil
}

func verifySecretsHaveSameData(secretA *apicorev1.Secret, secretB *apicorev1.Secret) error {
	for k, d := range secretA.Data {
		if !bytes.Equal(d, secretB.Data[k]) {
			return errNotSyncedSecret
		}
	}
	return nil
}

func fetchCertificateSecret(ctx context.Context, secretName types.NamespacedName, k8sClient client.Client,
) (*apicorev1.Secret, error) {
	certificateSecret := &apicorev1.Secret{}
	if err := k8sClient.Get(ctx,
		secretName,
		certificateSecret,
	); err != nil {
		return nil, fmt.Errorf("failed to fetch kcp certificate secret %w", err)
	}

	return certificateSecret, nil
}

func DeleteCertificateSecret(ctx context.Context, secret types.NamespacedName, k8sClient client.Client,
) error {
	certificateSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	}
	err := k8sClient.Delete(ctx, certificateSecret)
	if err != nil {
		return fmt.Errorf("failed to delete secret %w", err)
	}

	return nil
}

func GetCertificate(ctx context.Context, namespacedCertName types.NamespacedName, k8sClient client.Client,
) (*certmanagerv1.Certificate, error) {
	cert := &certmanagerv1.Certificate{}
	if err := k8sClient.Get(ctx, namespacedCertName, cert); err != nil {
		return nil, fmt.Errorf("failed to get secret %w", err)
	}

	return cert, nil
}

func GatewaySecretCreationTimeIsUpdated(ctx context.Context, oldTime time.Time, kcpClient client.Client) error {
	gwSecret, err := GetGatewaySecret(ctx, kcpClient)
	if err != nil {
		return err
	}

	currentTime, err := GetLastCaBundleExtendedTimeFromAnnotation(gwSecret)
	if err != nil {
		return fmt.Errorf("failed to get last modified time %w", err)
	}
	if currentTime.After(oldTime) {
		return nil
	}
	return errCreationTimeNotUpdated
}

func GetGatewaySecret(ctx context.Context, clnt client.Client) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{
		Name:      shared.GatewaySecretName,
		Namespace: shared.IstioNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get gateway secret %s: %w", shared.GatewaySecretName, err)
	}
	return secret, nil
}

func GetLastCaBundleExtendedTimeFromAnnotation(secret *apicorev1.Secret) (time.Time, error) {
	if gwSecretCaAddedToBundleAtValue, ok := secret.Annotations[shared.CaAddedToBundleAtAnnotation]; ok {
		if gwSecretCaAddedToBundleTime, err := time.Parse(time.RFC3339, gwSecretCaAddedToBundleAtValue); err == nil {
			return gwSecretCaAddedToBundleTime, nil
		}
	}
	return time.Time{}, errors.New("getting caAddedToBundleAt time failed")
}

func RotateCAManuallyWithGCM(ctx context.Context, kcpClient client.Client) error {
	cert := &gcertv1alpha1.Certificate{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.CACertificateName,
			Namespace: shared.IstioNamespace,
		},
	}

	patch := []byte(`{
        "spec": {
            "ensureRenewedAfter": null,
            "renew": true
        }
    }`)

	if err := kcpClient.Patch(ctx, cert, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return fmt.Errorf("failed to patch CA certificate: %w", err)
	}
	return nil
}

func UpdateGatewaySecretWithExpiringCert(ctx context.Context, k8sClient client.Client) error {
	certPEM, err := generateSelfSignedCertPEM(time.Now().Add(-1*time.Hour), time.Now().Add(7*24*time.Hour))
	if err != nil {
		return err
	}

	secret, err := GetGatewaySecret(ctx, k8sClient)
	if err != nil {
		return err
	}

	patch := secret.DeepCopy()
	patch.Data[apicorev1.TLSCertKey] = certPEM
	return k8sClient.Patch(ctx, patch, client.MergeFrom(secret))
}

func generateSelfSignedCertPEM(notBefore, notAfter time.Time) ([]byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}
