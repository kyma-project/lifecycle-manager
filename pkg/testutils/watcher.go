package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errOldCreationTime     = errors.New("certificate has an old creation timestamp")
	errNotSyncedSecret     = errors.New("secret is not synced to skr cluster")
	errTlsSecretNotRotated = errors.New("tls secret did not rotated")
)

func CertificateSecretExists(ctx context.Context, secretName types.NamespacedName, k8sClient client.Client) error {
	certificateSecret := &apicorev1.Secret{}
	err := k8sClient.Get(ctx, secretName, certificateSecret)
	if err != nil {
		return fmt.Errorf("failed to get certificate secret %w", err)
	}

	return nil
}

func CertificateSecretIsCreatedAfter(ctx context.Context,
	secretName types.NamespacedName, k8sClient client.Client, notBeforeTime *apimetav1.Time,
) error {
	certificateSecret, err := fetchCertificateSecret(ctx, secretName, k8sClient)
	if err != nil {
		return fmt.Errorf("failed to fetch certificate secret %w", err)
	}

	if certificateSecret.CreationTimestamp.Before(notBeforeTime) {
		return errOldCreationTime
	}

	return nil
}

func TlsSecretRotated(ctx context.Context, oldValue time.Time,
	namespacedSecretName types.NamespacedName, kcpClient client.Client,
) error {
	secret, err := GetTlsSecret(ctx, namespacedSecretName, kcpClient)
	if err != nil {
		return fmt.Errorf("failed to fetch tls secret: %w", err)
	}
	if secret.CreationTimestamp.Time == oldValue {
		return errTlsSecretNotRotated
	}
	return nil
}

func CertificateSecretIsSyncedToSkrCluster(ctx context.Context,
	kcpSecretName types.NamespacedName, kcpClient client.Client,
	skrSecretName types.NamespacedName, skrClient client.Client,
) error {
	kcpCertificateSecret, err := fetchCertificateSecret(ctx, kcpSecretName, kcpClient)
	if err != nil {
		return fmt.Errorf("failed to fetch kcp certificate secret %w", err)
	}

	skrCertificateSecret, err := fetchCertificateSecret(ctx, skrSecretName, skrClient)
	if err != nil {
		return fmt.Errorf("failed to fetch kcp certificate secret %w", err)
	}

	for k, d := range kcpCertificateSecret.Data {
		if !bytes.Equal(d, skrCertificateSecret.Data[k]) {
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

func GetCACertificate(ctx context.Context, namespacedCertName types.NamespacedName, k8sClient client.Client,
) (*certmanagerv1.Certificate, error) {
	caCert := &certmanagerv1.Certificate{}
	if err := k8sClient.Get(ctx, namespacedCertName, caCert); err != nil {
		return nil, fmt.Errorf("failed to get secret %w", err)
	}

	return caCert, nil
}

func GetTlsSecret(ctx context.Context, namespacedSecretName types.NamespacedName, k8sClient client.Client,
) (*apicorev1.Secret, error) {
	tlsSecret := &apicorev1.Secret{}
	if err := k8sClient.Get(ctx, namespacedSecretName, tlsSecret); err != nil {
		return nil, fmt.Errorf("failed to get secret %w", err)
	}

	return tlsSecret, nil
}
