package skrcontextimpl_test

import (
	"context"
	"errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"runtime"
	"sync"
	"testing"
	"time"

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

	_, err := dualFactory.Get(types.NamespacedName{Name: "kymaUninitialized"})

	require.Error(t, err)
	assert.ErrorIs(t, err, testskrcontext.ErrSkrEnvNotStarted)
}

func Test_StopWithErrors(t *testing.T) {
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
	assert.True(t, envPrimary.stopCalled)
	assert.True(t, envSecondary.stopCalled)
	assert.True(t, envTertiary.stopCalled)
}

func Test_StopIdempotent(t *testing.T) {
	dualFactory := newFactory()
	fakeEnv := &fakeEnv{name: "test-env"}
	dualFactory.StoreEnv("test-env", fakeEnv)

	require.NoError(t, dualFactory.Stop())

	assert.True(t, fakeEnv.stopCalled)
}

func Test_StopClearsAllEntries(t *testing.T) {
	dualFactory := newFactory()
	for range make([]struct{}, 5) {
		fakeEnv := &fakeEnv{name: "test-env"}
		dualFactory.StoreEnv("test-env", fakeEnv)
	}

	require.NoError(t, dualFactory.Stop())

	assert.Nil(t, dualFactory.GetSkrEnv())
	_, err := dualFactory.Get(types.NamespacedName{Name: "test-env"})
	assert.Error(t, err)
}

func Test_ConcurrentStopCalls(t *testing.T) {
	dualFactory := newFactory()
	for range make([]struct{}, 10) {
		fakeEnv := &fakeEnv{name: "test-env"}
		dualFactory.StoreEnv("test-env", fakeEnv)
	}
	var waitGroup sync.WaitGroup
	errors := make(chan error, 5)

	for range make([]struct{}, 5) {
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

func Test_LeakPrevention_VerifyNoGoroutineLeaks(t *testing.T) {
	before := runtime.NumGoroutine()
	for range make([]struct{}, 3) {
		dualFactory := newFactory()
		for range make([]struct{}, 5) {
			fakeEnv := &leakyFakeEnv{
				name:                 "test-env",
				shouldSpawnGoroutine: true,
			}
			dualFactory.StoreEnv("test-env", fakeEnv)
		}
		require.NoError(t, dualFactory.Stop())
		time.Sleep(10 * time.Millisecond)
	}

	// Force garbage collection to clean up any lingering references
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, before+2,
		"Expected no significant goroutine leaks. Before: %d, After: %d", before, after)
}

func Test_LeakPrevention_VerifyStopperInterfaceHandling(t *testing.T) {
	dualFactory := newFactory()
	normalStopper := &fakeEnv{name: "normal"}
	errorStopper := &fakeEnv{name: "error", stopErr: errors.New("stop error")}
	leakyStopper := &leakyFakeEnv{name: "leaky", shouldSpawnGoroutine: true}
	dualFactory.StoreEnv("normal", normalStopper)
	dualFactory.StoreEnv("error", errorStopper)
	dualFactory.StoreEnv("leaky", leakyStopper)
	dualFactory.StoreEnv("non-stopper", "this is just a string")

	err := dualFactory.Stop()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stop error")
	assert.True(t, normalStopper.stopCalled)
	assert.True(t, errorStopper.stopCalled)
	assert.True(t, leakyStopper.stopCalled)
}

type fakeEnv struct {
	name       string
	stopCalled bool
	stopErr    error
}

func (f *fakeEnv) Stop() error {
	f.stopCalled = true
	return f.stopErr
}

type leakyFakeEnv struct {
	name                 string
	stopCalled           bool
	stopErr              error
	shouldSpawnGoroutine bool
	cancel               context.CancelFunc
}

func (l *leakyFakeEnv) Stop() error {
	l.stopCalled = true

	if l.shouldSpawnGoroutine && l.cancel == nil {
		// Simulate starting a background goroutine (like envtest)
		ctx, cancel := context.WithCancel(context.Background())
		l.cancel = cancel

		go func() {
			<-ctx.Done() // Wait for cancellation
		}()
	}

	// Clean up the goroutine
	if l.cancel != nil {
		l.cancel()
	}

	return l.stopErr
}
