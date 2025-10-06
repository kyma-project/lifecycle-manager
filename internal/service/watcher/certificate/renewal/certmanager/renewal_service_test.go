package certmanager_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	certmanagerrenewal "github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal/certmanager" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRenew_WhenRepositoryDeleteSucceeds_Returns(t *testing.T) {
	renewalService := certmanagerrenewal.NewService(&secretRepoStub{})

	err := renewalService.Renew(t.Context(), random.Name())

	require.NoError(t, err)
}

func TestRenew_WhenRepositoryDeleteReturnsAnError_ReturnsError(t *testing.T) {
	renewalService := certmanagerrenewal.NewService(&secretRepoStub{
		err: assert.AnError,
	})

	err := renewalService.Renew(t.Context(), random.Name())

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "failed to renew SKR certificate secret. Deletion failed")
}

func TestSkrSecretNeedsRenewal_WhenSkrCreationOlderThanGatewayLastModified_ReturnsTrue(t *testing.T) {
	renewalService := certmanagerrenewal.NewService(&secretRepoStub{})
	gatewaySecret := &apicorev1.Secret{ // gateway secret, modified now
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: time.Now().Format(time.RFC3339),
			},
		},
	}
	skrSecret := &apicorev1.Secret{ // skr secret, created a minute ago
		ObjectMeta: apimetav1.ObjectMeta{
			CreationTimestamp: apimetav1.Time{
				Time: time.Now().Add(-time.Minute),
			},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

func TestSkrSecretNeedsRenewal_WhenSkrCreationNewerThanGatewayLastModified_ReturnsFalse(t *testing.T) {
	renewalService := certmanagerrenewal.NewService(&secretRepoStub{})
	gatewaySecret := &apicorev1.Secret{ // gateway secret, modified a minute ago
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: time.Now().Add(-time.Minute).Format(time.RFC3339),
			},
		},
	}
	skrSecret := &apicorev1.Secret{ // skr secret, created now
		ObjectMeta: apimetav1.ObjectMeta{
			CreationTimestamp: apimetav1.Time{
				Time: time.Now(),
			},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.False(t, result)
}

func TestSkrSecretNeedsRenewal_GatewaySecretHasNoLastModified_ReturnsTrue(t *testing.T) {
	renewalService := certmanagerrenewal.NewService(&secretRepoStub{})
	// gateway secret, no last modified
	gatewaySecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}
	// skr secret, created a minute ago
	skrSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			CreationTimestamp: apimetav1.Time{
				Time: time.Now().Add(-time.Minute),
			},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

func TestRenewSkrCertificate_WhenGatewaySecretHasInvalidLastModified_ReturnsTrue(t *testing.T) {
	renewalService := certmanagerrenewal.NewService(&secretRepoStub{})
	// gateway secret, invalid last modified
	gatewaySecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: "not a time",
			},
		},
	}
	// skr secret, created a minute ago
	skrSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			CreationTimestamp: apimetav1.Time{
				Time: time.Now().Add(-time.Minute),
			},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

type secretRepoStub struct {
	err error
}

func (s *secretRepoStub) Delete(_ context.Context, _ string) error {
	if s.err != nil {
		return s.err
	}
	return nil
}
