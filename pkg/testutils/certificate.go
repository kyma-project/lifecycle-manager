package testutils

import (
	"context"
	"fmt"
	"time"

	certmanagerapplyv1 "github.com/cert-manager/cert-manager/pkg/client/applyconfigurations/certmanager/v1"
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
	certApply := certmanagerapplyv1.Certificate(cert.Name, cert.Namespace).
		WithStatus(certmanagerapplyv1.CertificateStatus().
			WithNotBefore(apimetav1.Time{Time: notBeforeTime}).
			WithNotAfter(apimetav1.Time{Time: notAfterTime}),
		)

	if err := kcpClient.Status().Apply(
		ctx,
		certApply,
		fieldowners.LifecycleManager,
	); err != nil {
		return fmt.Errorf("failed to add NotBefore to certificate status: %w", err)
	}

	return nil
}
