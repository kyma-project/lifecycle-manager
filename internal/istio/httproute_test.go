package istio_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_NewHTTPRoute_ReturnsError_WhenWatcherIsNil(t *testing.T) {
	var watcher *v1beta2.Watcher = nil

	httpRoute, err := istio.NewHTTPRoute(watcher)

	assert.Nil(t, httpRoute)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher")
}

func Test_NewHTTPRoute_ReturnsError_WhenNoName(t *testing.T) {
	watcher := builder.NewWatcherBuilder().
		WithName("").
		Build()

	httpRoute, err := istio.NewHTTPRoute(watcher)

	assert.Nil(t, httpRoute)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Name")
}

func Test_NewHTTPRoute_ReturnsError_WhenNoNamespace(t *testing.T) {
	watcher := builder.NewWatcherBuilder().
		WithNamespace("").
		Build()

	httpRoute, err := istio.NewHTTPRoute(watcher)

	assert.Nil(t, httpRoute)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Namespace")
}

func Test_NewHTTPRoute_ReturnsError_WhenNoModuleName(t *testing.T) {
	watcher := builder.NewWatcherBuilder().Build()
	watcher.Labels = nil

	httpRoute, err := istio.NewHTTPRoute(watcher)

	assert.Nil(t, httpRoute)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "GetManagerName()")
}

func Test_NewHTTPRoute_ReturnsError_WhenNoServiceInfoName(t *testing.T) {
	watcher := builder.NewWatcherBuilder().
		WithServiceInfoName("").
		Build()

	httpRoute, err := istio.NewHTTPRoute(watcher)

	assert.Nil(t, httpRoute)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Spec.ServiceInfo.Name")
}

func Test_NewHTTPRoute_ReturnsError_WhenNoServiceInfoNamespace(t *testing.T) {
	watcher := builder.NewWatcherBuilder().
		WithServiceInfoNamespace("").
		Build()

	httpRoute, err := istio.NewHTTPRoute(watcher)

	assert.Nil(t, httpRoute)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Spec.ServiceInfo.Namespace")
}

func Test_NewHTTPRoute_ReturnsError_WhenNoServiceInfoPort(t *testing.T) {
	watcher := builder.NewWatcherBuilder().
		WithServiceInfoPort(0).
		Build()

	httpRoute, err := istio.NewHTTPRoute(watcher)

	assert.Nil(t, httpRoute)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Spec.ServiceInfo.Port")
}

func Test_NewHTTPRoute_ReturnsCorrectHttpRoute(t *testing.T) {
	watcher := builder.NewWatcherBuilder().Build()
	expectedHTTPRouteName := getWatcherName(watcher)
	expectedHTTPRouteMatchURIPrefix := getHTTPRoutePrefix(watcher)
	expectedHTTPRouteDestinationHost := getDestinationHost(watcher)
	expectedHTTPRouteDestinationPort := getDestinationPort(watcher)

	httpRoute, err := istio.NewHTTPRoute(watcher)

	require.NoError(t, err)
	assert.Equal(t, expectedHTTPRouteName, httpRoute.GetName())
	assert.Equal(t, expectedHTTPRouteMatchURIPrefix, httpRoute.GetMatch()[0].GetUri().GetPrefix())
	assert.Equal(t, expectedHTTPRouteDestinationHost, httpRoute.GetRoute()[0].GetDestination().GetHost())
	assert.Equal(t, expectedHTTPRouteDestinationPort, httpRoute.GetRoute()[0].GetDestination().GetPort().GetNumber())
}

func getWatcherName(watcher *v1beta2.Watcher) string {
	return fmt.Sprintf("%v/%v", watcher.Namespace, watcher.Name)
}

func getHTTPRoutePrefix(watcher *v1beta2.Watcher) string {
	return fmt.Sprintf("/v2/%s/event", watcher.GetLabels()[shared.ManagedBy])
}

func getDestinationHost(watcher *v1beta2.Watcher) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", watcher.Spec.ServiceInfo.Name, watcher.Spec.ServiceInfo.Namespace)
}

func getDestinationPort(watcher *v1beta2.Watcher) uint32 {
	return uint32(watcher.Spec.ServiceInfo.Port) //nolint:gosec // test code
}
