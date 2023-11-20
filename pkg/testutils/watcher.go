package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errOldCreationTime = errors.New("certificate has an old creation timestamp")
	errNotSynedSecret  = errors.New("secret is not synced to skr cluster")
)

func CertificateSecretExists(ctx context.Context,
	secretName, secretNamespace string, k8sClient client.Client,
) error {
	certificateSecret := &apicorev1.Secret{}
	err := k8sClient.Get(ctx,
		client.ObjectKey{Name: secretName, Namespace: secretNamespace},
		certificateSecret,
	)
	if err != nil {
		return fmt.Errorf("failed to get certificate secret %w", err)
	}

	return nil
}

func CertificateSecretIsCreatedAfter(ctx context.Context,
	secretName, secretNamespace string, k8sClient client.Client, notBeforeTime *apimetav1.Time,
) error {
	certificateSecret, err := fetchCertificateSecret(ctx, secretName, secretNamespace, k8sClient)
	if err != nil {
		return fmt.Errorf("failed to fetch certificate secret %w", err)
	}

	if certificateSecret.CreationTimestamp.Before(notBeforeTime) {
		return errOldCreationTime
	}

	return nil
}

func CertificateSecretIsSyncedToSkrCluster(ctx context.Context,
	kcpSecretName, kcpSecretNamespace string, controlPlaneClient client.Client, skrSecretName,
	skrSecretNamespace string, runtimeClient client.Client,
) error {
	kcpCertificateSecret, err := fetchCertificateSecret(ctx, kcpSecretName, kcpSecretNamespace, controlPlaneClient)
	if err != nil {
		return fmt.Errorf("failed to fetch kcp certificate secret %w", err)
	}

	skrCertificateSecret, err := fetchCertificateSecret(ctx, skrSecretName, skrSecretNamespace, runtimeClient)
	if err != nil {
		return fmt.Errorf("failed to fetch kcp certificate secret %w", err)
	}

	for k, d := range kcpCertificateSecret.Data {
		if !bytes.Equal(d, skrCertificateSecret.Data[k]) {
			return errNotSynedSecret
		}
	}

	return nil
}

func fetchCertificateSecret(ctx context.Context, secretName string, secretNamespace string, k8sClient client.Client,
) (*apicorev1.Secret, error) {
	certificateSecret := &apicorev1.Secret{}
	if err := k8sClient.Get(ctx,
		client.ObjectKey{Name: secretName, Namespace: secretNamespace},
		certificateSecret,
	); err != nil {
		return nil, fmt.Errorf("failed to fetch kcp certificate secret %w", err)
	}

	return certificateSecret, nil
}

func DeleteCertificateSecret(ctx context.Context,
	secretName, secretNamespace string, k8sClient client.Client,
) error {
	certificateSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}
	err := k8sClient.Delete(ctx, certificateSecret)
	if err != nil {
		return fmt.Errorf("failed to delete secret %w", err)
	}

	return nil
}

func GetCACertificate(ctx context.Context,
	secretName, secretNamespace string, k8sClient client.Client,
) (*certmanagerv1.Certificate, error) {
	caCert := &certmanagerv1.Certificate{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: secretNamespace, Name: secretName}, caCert); err != nil {
		return nil, fmt.Errorf("failed to get secret %w", err)
	}

	return caCert, nil
}
