package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func Test_DeleteMetrics_IsApplicable_MetricsExist(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	metricsStub := &kymaMetricsStub{
		hasMetrics: true,
	}

	uc := usecases.NewDeleteMetrics(metricsStub)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, metricsStub.hasMetricsCalled)
	assert.Equal(t, kcpKyma.GetName(), metricsStub.kymaName)
}

func Test_DeleteMetrics_IsApplicable_MetricsDoNotExist(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	metricsStub := &kymaMetricsStub{
		hasMetrics: false,
	}

	uc := usecases.NewDeleteMetrics(metricsStub)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, metricsStub.hasMetricsCalled)
	assert.Equal(t, kcpKyma.GetName(), metricsStub.kymaName)
}

func Test_DeleteMetrics_IsApplicable_HasMetricsFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	metricsStub := &kymaMetricsStub{
		hasMetricsErr: assert.AnError,
	}

	uc := usecases.NewDeleteMetrics(metricsStub)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, applicable)
	assert.True(t, metricsStub.hasMetricsCalled)
	assert.Equal(t, kcpKyma.GetName(), metricsStub.kymaName)
}

func Test_DeleteMetrics_Execute_CleanupSucceeds(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	metricsStub := &kymaMetricsStub{}

	uc := usecases.NewDeleteMetrics(metricsStub)

	res := uc.Execute(t.Context(), kcpKyma)

	require.NoError(t, res.Err)
	assert.Equal(t, usecase.DeleteMetrics, res.UseCase)
	assert.True(t, metricsStub.cleanupMetricsCalled)
	assert.Equal(t, kcpKyma.GetName(), metricsStub.kymaName)
}

type kymaMetricsStub struct {
	usecases.KymaMetrics

	hasMetricsCalled     bool
	cleanupMetricsCalled bool
	kymaName             string
	hasMetrics           bool
	hasMetricsErr        error
}

func (m *kymaMetricsStub) HasMetrics(kymaName string) (bool, error) {
	m.hasMetricsCalled = true
	m.kymaName = kymaName
	return m.hasMetrics, m.hasMetricsErr
}

func (m *kymaMetricsStub) CleanupMetrics(kymaName string) {
	m.cleanupMetricsCalled = true
	m.kymaName = kymaName
}
