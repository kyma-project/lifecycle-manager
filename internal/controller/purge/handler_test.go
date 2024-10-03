//nolint:testpackage // This handler doesn't have exported methods, so I want to test the unexported ones
package purge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_reconcileHandler_getKyma_unknownError(t *testing.T) {
	// given
	mockErr := errors.New("unknown-error")
	getFn := func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		return mockErr
	}

	r := reconcileHandler{
		Get: getFn, // Advantage of using a function value instead of an interface is that we can easily mock just a single method (instead having to mock all of them)
	}
	// when
	_, err := r.reconcile(context.Background(), ctrl.Request{})

	// then
	require.Error(t, err)
	require.ErrorIs(t, err, mockErr)
	assert.Contains(t, err.Error(), "failed getting Kyma")
}
