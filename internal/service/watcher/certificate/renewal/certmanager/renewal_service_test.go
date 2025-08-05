package certmanager_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	certmanagerrenewal "github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal/certmanager"
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

type secretRepoStub struct {
	err error
}

func (s *secretRepoStub) Delete(_ context.Context, _ string) error {
	if s.err != nil {
		return s.err
	}
	return nil
}
