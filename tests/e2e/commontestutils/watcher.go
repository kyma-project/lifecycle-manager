package commontestutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	err := k8sClient.Get(ctx,
		secretName,
		certificateSecret,
	)
	if err != nil {
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

func GetCACertificate(ctx context.Context, namespacedCertName types.NamespacedName, k8sClient client.Client,
) (*certmanagerv1.Certificate, error) {
	caCert := &certmanagerv1.Certificate{}
	err := k8sClient.Get(ctx, namespacedCertName, caCert)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %w", err)
	}

	return caCert, nil
}

func GatewaySecretCreationTimeIsUpdated(ctx context.Context, oldTime time.Time, kcpClient client.Client) error {
	gwSecret, err := GetGatewaySecret(ctx, kcpClient)
	if err != nil {
		return err
	}

	currentTime, err := GetLastModifiedTimeFromAnnotation(gwSecret)
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
	err := clnt.Get(ctx, client.ObjectKey{
		Name:      shared.GatewaySecretName,
		Namespace: shared.IstioNamespace,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway secret %s: %w", shared.GatewaySecretName, err)
	}
	return secret, nil
}

func GetLastModifiedTimeFromAnnotation(secret *apicorev1.Secret) (time.Time, error) {
	if gwSecretLastModifiedAtValue, ok := secret.Annotations[shared.LastModifiedAtAnnotation]; ok {
		gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue)
		if err == nil {
			return gwSecretLastModifiedAt, nil
		}
	}
	return time.Time{}, errors.New("getting lastModifiedAt time failed")
}

func RotateCAManuallyWithGCM(ctx context.Context, kcpClient client.Client) error {
	caCert := &gcertv1alpha1.Certificate{}
	err := kcpClient.Get(ctx, types.NamespacedName{
		Name:      shared.CACertificateName,
		Namespace: shared.IstioNamespace,
	}, caCert)
	if err != nil {
		return fmt.Errorf("failed to get CA certificate %w", err)
	}
	caCert.Spec.EnsureRenewedAfter = nil
	renew := true
	caCert.Spec.Renew = &renew
	err := kcpClient.Update(ctx, caCert)
	if err != nil {
		return fmt.Errorf("failed to update CA certificate %w", err)
	}
	return nil
}
