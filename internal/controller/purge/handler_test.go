package purge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileHandler_kyma_not_found(t *testing.T) {
	// given
	mockErr := errors.New("mocked-not-found")
	getFn := func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		return mockErr
	}

	r := reconcileHandler{
		Get: getFn, // Advantage of using a function value instead of an interface is that we can easily mock just a single method (instead having to mock all of them)
	}
	// when
	_, err := r.reconcile(context.Background(), ctrl.Request{})

	// then
	assert.Error(t, err)
	assert.True(t, errors.Is(err, mockErr))
}
