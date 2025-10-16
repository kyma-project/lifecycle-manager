package usecases_test

import (
	"context"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/deletion"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
)

func TestSkipNonMandatory_WithNonMandatoryModule(t *testing.T) {
	t.Parallel()

	skipNonMandatory := usecases.NewSkipNonMandatory()
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Mandatory: nil,
		},
	}

	isApplicable, err := skipNonMandatory.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.True(t, isApplicable)

	executeErr := skipNonMandatory.Execute(context.Background(), mrm)
	require.Error(t, executeErr)
	require.Equal(t, deletion.ErrMrmNotMandatory, executeErr)
}

func TestSkipNonMandatory_WithMandatoryModule(t *testing.T) {
	t.Parallel()

	skipNonMandatory := usecases.NewSkipNonMandatory()
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Mandatory: &v1beta2.Mandatory{
				Version: "1.0.0",
			},
		},
	}

	isApplicable, err := skipNonMandatory.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, isApplicable)
}
