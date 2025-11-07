package skrsync_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrsync"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestSyncImagePullSecret_WhenSecretNameIsNotConfigured_ReturnsError(t *testing.T) {
	skrSyncService := skrsync.NewService(nil, nil, nil, "")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.ErrorIs(t, err, skrsync.ErrImagePullSecretNotConfigured)
}

func TestSyncCrds_WhenCalled_ShouldInvokeUseCase(t *testing.T) {
	syncCrdsUseCase := &syncCrdsUseCaseStub{}
	skrSyncService := skrsync.NewService(nil, nil, syncCrdsUseCase, "")

	_, err := skrSyncService.SyncCrds(t.Context(), &v1beta2.Kyma{})

	require.NoError(t, err)
	require.True(t, syncCrdsUseCase.called)
}

type syncCrdsUseCaseStub struct {
	called bool
}

func (s *syncCrdsUseCaseStub) Execute(_ context.Context, _ *v1beta2.Kyma) (bool, error) {
	s.called = true
	return false, nil
}
