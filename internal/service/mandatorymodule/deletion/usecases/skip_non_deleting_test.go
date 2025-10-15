package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSkipNonDeleting_NotDeleting(t *testing.T) {
	t.Parallel()

	skipNonDeleting := usecases.NewSkipNonDeleting()
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-module",
		},
	}

	shouldExecute, err := skipNonDeleting.ShouldExecute(context.Background(), mrm)
	require.NoError(t, err)
	require.True(t, shouldExecute)

	executeErr := skipNonDeleting.Execute(context.Background(), mrm)
	require.ErrorIs(t, executeErr, deletion.ErrMrmNotInDeletingState)
}

func TestSkipNonDeleting_IsDeleting(t *testing.T) {
	t.Parallel()

	skipNonDeleting := usecases.NewSkipNonDeleting()
	now := metav1.NewTime(time.Now())
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-module",
			DeletionTimestamp: &now,
		},
	}

	shouldExecute, err := skipNonDeleting.ShouldExecute(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, shouldExecute)
}
