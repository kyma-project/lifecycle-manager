package testutils

import (
	"context"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
)

func AddValidityToCertificateStatus(ctx context.Context,
	kcpClient client.Client,
	cert client.ObjectKey,
	notBeforeTime time.Time,
	notAfterTime time.Time,
) error {
	certificate := &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      cert.Name,
			Namespace: cert.Namespace,
		},
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: notBeforeTime,
			},
			NotAfter: &apimetav1.Time{
				Time: notAfterTime,
			},
		},
	}

	if err := kcpClient.Status().Patch(
		ctx,
		certificate,
		client.Apply,
		fieldowners.LifecycleManager,
	); err != nil {
		return fmt.Errorf("failed to add NotBefore to certificate status: %w", err)
	}

	return nil
}
