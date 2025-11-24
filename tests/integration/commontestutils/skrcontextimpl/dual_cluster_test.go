package skrcontextimpl_test

import (
	"errors"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	testskrcontext "github.com/kyma-project/lifecycle-manager/tests/integration/commontestutils/skrcontextimpl"
)

func TestDualClusterFactory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DualCluster Factory Suite")
}

func newFactory() *testskrcontext.DualClusterFactory {
	scheme := machineryruntime.NewScheme()
	return testskrcontext.NewDualClusterFactory(scheme, nil)
}

func Test_GetBeforeInit(t *testing.T) {
	dualFactory := newFactory()

	_, err := dualFactory.Get(t.Context(), types.NamespacedName{Name: "kymaUninitialized"})

	require.Error(t, err)
	assert.ErrorIs(t, err, testskrcontext.ErrSkrEnvNotStarted)
}

func Test_StoreEnv_ValidatesInput(t *testing.T) {
	dualFactory := newFactory()

	err := dualFactory.StoreEnv("", &envtest.Environment{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func Test_StopWithErrors(t *testing.T) {
	dualFactory := newFactory()

	// Store real envtest.Environment first, then replace with test doubles
	require.NoError(t, dualFactory.StoreEnv("primary-env", &envtest.Environment{}))
	require.NoError(t, dualFactory.StoreEnv("secondary-env", &envtest.Environment{}))
	require.NoError(t, dualFactory.StoreEnv("tertiary-env", &envtest.Environment{}))

	// Create test doubles
	envPrimary := &fakeEnvTest{name: "primary-env", stopErr: errors.New("primary stop failure")}
	envSecondary := &fakeEnvTest{name: "secondary-env", stopErr: errors.New("secondary stop failure")}
	envTertiary := &fakeEnvTest{name: "tertiary-env"}

	// Replace with test doubles directly in the map
	dualFactory.SkrEnvs.Store("primary-env", envPrimary)
	dualFactory.SkrEnvs.Store("secondary-env", envSecondary)
	dualFactory.SkrEnvs.Store("tertiary-env", envTertiary)

	err := dualFactory.Stop()

	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "primary stop failure")
	assert.Contains(t, msg, "secondary stop failure")
	assert.True(t, envPrimary.stopCalled)
	assert.True(t, envSecondary.stopCalled)
	assert.True(t, envTertiary.stopCalled)
}

func Test_StopClearsAllEntriesAndIsIdempotent(t *testing.T) {
	dualFactory := newFactory()

	require.NoError(t, dualFactory.StoreEnv("test-env", &envtest.Environment{}))

	fakeEnv := &fakeEnvTest{name: "test-env"}
	dualFactory.SkrEnvs.Store("test-env", fakeEnv)

	require.NoError(t, dualFactory.Stop())
	assert.True(t, fakeEnv.stopCalled)
	assert.Nil(t, dualFactory.GetSkrEnv())

	// Verify entry is cleared
	_, err := dualFactory.Get(t.Context(), types.NamespacedName{Name: "test-env"})
	require.Error(t, err)

	// Second stop should also succeed (idempotent)
	require.NoError(t, dualFactory.Stop())
}

func Test_ConcurrentStopCalls(t *testing.T) {
	dualFactory := newFactory()

	for range 10 {
		require.NoError(t, dualFactory.StoreEnv("test-env", &envtest.Environment{}))
	}

	fakeEnv := &fakeEnvTest{name: "test-env"}
	dualFactory.SkrEnvs.Store("test-env", fakeEnv)

	var waitGroup sync.WaitGroup
	errors := make(chan error, 5)

	for range 5 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errors <- dualFactory.Stop()
		}()
	}
	waitGroup.Wait()
	close(errors)

	for err := range errors {
		assert.NoError(t, err)
	}
}

func Test_StopperInterfaceHandling(t *testing.T) {
	dualFactory := newFactory()

	require.NoError(t, dualFactory.StoreEnv("normal", &envtest.Environment{}))
	require.NoError(t, dualFactory.StoreEnv("error", &envtest.Environment{}))

	normalStopper := &fakeEnvTest{name: "normal"}
	errorStopper := &fakeEnvTest{name: "error", stopErr: errors.New("stop error")}

	dualFactory.SkrEnvs.Store("normal", normalStopper)
	dualFactory.SkrEnvs.Store("error", errorStopper)

	err := dualFactory.Stop()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stop error")
	assert.True(t, normalStopper.stopCalled)
	assert.True(t, errorStopper.stopCalled)
}

type fakeEnvTest struct {
	name       string
	stopCalled bool
	stopErr    error
}

func (f *fakeEnvTest) Stop() error {
	f.stopCalled = true
	return f.stopErr
}
