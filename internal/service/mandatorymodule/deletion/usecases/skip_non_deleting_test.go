package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/deletion"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
)

func TestSkipNonDeleting_NotDeleting(t *testing.T) {
	t.Parallel()

	skipNonDeleting := usecases.NewSkipNonDeleting()
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "test-module",
		},
	}

	isApplicable, err := skipNonDeleting.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.True(t, isApplicable)

	executeErr := skipNonDeleting.Execute(context.Background(), mrm)
	require.ErrorIs(t, executeErr, deletion.ErrMrmNotInDeletingState)
}

func TestSkipNonDeleting_IsDeleting(t *testing.T) {
	t.Parallel()

	skipNonDeleting := usecases.NewSkipNonDeleting()
	now := apimetav1.NewTime(time.Now())
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              "test-module",
			DeletionTimestamp: &now,
		},
	}

	isApplicable, err := skipNonDeleting.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, isApplicable)
}
