package skrcontextimpl_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	testskrcontext "github.com/kyma-project/lifecycle-manager/tests/integration/commontestutils/skrcontextimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFactory() *testskrcontext.DualClusterFactory {
	scheme := machineryruntime.NewScheme()
	return testskrcontext.NewDualClusterFactory(scheme, nil)
}

func Test_GetBeforeInit(t *testing.T) {
	dualFactory := newFactory()
	_, err := dualFactory.Get(types.NamespacedName{Name: "kymaUninitialized"})
	require.Error(t, err)
	assert.ErrorIs(t, err, testskrcontext.ErrSkrEnvNotStarted)
}

func Test_InitAndGet(t *testing.T) {
	dualFactory := newFactory()
	kyma := types.NamespacedName{Name: "kymaInit"}
	require.NoError(t, dualFactory.Init(context.Background(), kyma))
	skrCtx, err := dualFactory.Get(kyma)
	require.NoError(t, err)
	require.NotNil(t, skrCtx)
	require.NotNil(t, dualFactory.GetSkrEnv())
}

func Test_InitTwiceSameKyma(t *testing.T) {
	dualFactory := newFactory()
	kyma := types.NamespacedName{Name: "kymaSame"}
	require.NoError(t, dualFactory.Init(context.Background(), kyma))
	envFirst := dualFactory.GetSkrEnv()
	require.NotNil(t, envFirst)
	require.NoError(t, dualFactory.Init(context.Background(), kyma))
	envSecond := dualFactory.GetSkrEnv()
	assert.Equal(t, envFirst, envSecond)
}

func Test_MultipleKymasAndStop(t *testing.T) {
	dualFactory := newFactory()
	kymaPrimary := types.NamespacedName{Name: "kymaPrimary"}
	kymaSecondary := types.NamespacedName{Name: "kymaSecondary"}

	for _, k := range []types.NamespacedName{kymaPrimary, kymaSecondary} {
		require.NoError(t, dualFactory.Init(context.Background(), k))
		_, err := dualFactory.Get(k)
		require.NoError(t, err)
	}

	require.NoError(t, dualFactory.Stop())
	assert.Nil(t, dualFactory.GetSkrEnv())

	_, err := dualFactory.Get(kymaPrimary)
	assert.Error(t, err)
}

func Test_StopIdempotent(t *testing.T) {
	dualFactory := newFactory()
	kyma := types.NamespacedName{Name: "kymaLifecycle"}
	require.NoError(t, dualFactory.Init(context.Background(), kyma))
	require.NoError(t, dualFactory.Stop())
	require.NoError(t, dualFactory.Stop())
}

func Test_NoLeakedProcesses(t *testing.T) {
	dualFactory := newFactory()
	kyma := types.NamespacedName{Name: "kymaGoroutineCheck"}

	before := runtime.NumGoroutine()
	require.NoError(t, dualFactory.Init(context.Background(), kyma))
	require.NoError(t, dualFactory.Stop())

	runtime.GC()
	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, before+2)
}

type fakeEnv struct {
	name       string
	stopCalled bool
	stopErr    error
}

func (fenv *fakeEnv) Stop() error {
	fenv.stopCalled = true
	return fenv.stopErr
}

func Test_StopAggregatesErrors(t *testing.T) {
	dualFactory := newFactory()

	envPrimary := &fakeEnv{name: "primary-env", stopErr: errors.New("primary stop failure")}
	envSecondary := &fakeEnv{name: "secondary-env", stopErr: errors.New("secondary stop failure")}
	envTertiary := &fakeEnv{name: "tertiary-env"}

	dualFactory.StoreEnv("primary-env", envPrimary)
	dualFactory.StoreEnv("secondary-env", envSecondary)
	dualFactory.StoreEnv("tertiary-env", envTertiary)

	err := dualFactory.Stop()
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "primary stop failure")
	assert.Contains(t, msg, "secondary stop failure")
	assert.Nil(t, dualFactory.GetSkrEnv())
}
