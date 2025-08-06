package istio_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func Test_NewVirtualServiceService_ReturnsError_WhenSchemeIsNil(t *testing.T) {
	vss, err := istio.NewVirtualServiceService(nil)

	assert.Nil(t, vss)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "scheme")
}

func Test_NewVirtualServiceService_ReturnsVirtualServiceService(t *testing.T) {
	scheme := createScheme(t)

	vss, err := istio.NewVirtualServiceService(scheme)

	assert.NotNil(t, vss)
	require.NoError(t, err)
}

func Test_NewVirtualService_ReturnsError_WhenWatcherIsNil(t *testing.T) {
	vss := createVirtualServiceService(t)
	var watcher *v1beta2.Watcher = nil
	gateways := createGateways()

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher")
}

func Test_NewVirtualService_ReturnsError_WhenWatcherNameIsEmpty(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().WithName("").Build()
	gateways := createGateways()

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Name")
}

func Test_NewVirtualService_SetsCorrectName(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()

	vs, err := vss.NewVirtualService(watcher, gateways)

	require.NoError(t, err)
	assert.Equal(t, watcher.Name, vs.Name)
}

func Test_NewVirtualService_ReturnsError_WhenNamespaceIsEmpty(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().WithNamespace("").Build()
	gateways := createGateways()

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Namespace")
}

func Test_NewVirtualService_SetsCorrectNamespace(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()

	vs, err := vss.NewVirtualService(watcher, gateways)

	require.NoError(t, err)
	assert.Equal(t, watcher.GetNamespace(), vs.Namespace)
}

func Test_NewVirtualService_ReturnsError_WhenGatewaysIsNil(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	var gateways *istioclientapiv1beta1.GatewayList = nil

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "gateways")
}

func Test_NewVirtualService_ReturnsError_WhenGatewaysAreEmpty(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := &istioclientapiv1beta1.GatewayList{}

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "gateways")
}

func Test_NewVirtualService_SetsCorrectGateways(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()
	expectedGatewayNames := getGatewayNamesMap(gateways.Items)

	vs, err := vss.NewVirtualService(watcher, gateways)

	require.NoError(t, err)
	assert.Len(t, vs.Spec.GetGateways(), len(expectedGatewayNames))
	for _, gateway := range vs.Spec.GetGateways() {
		_, exists := expectedGatewayNames[gateway]
		assert.True(t, exists)
	}
}

func Test_NewVirtualService_ReturnsError_WhenGatewaysHaveNoServers(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()
	for _, gateway := range gateways.Items {
		gateway.Spec.Servers = []*istioapiv1beta1.Server{}
	}

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	require.ErrorIs(t, err, istio.ErrCantFindGatewayServersHost)
}

func Test_NewVirtualService_ReturnsError_WhenGatewayServersHaveNoHosts(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()
	for _, gateway := range gateways.Items {
		for _, server := range gateway.Spec.GetServers() {
			server.Hosts = []string{}
		}
	}

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	require.ErrorIs(t, err, istio.ErrCantFindGatewayServersHost)
}

func Test_NewVirtualService_SetsCorrectHosts(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()
	expectedHosts := getHostNamesMap(gateways.Items)

	vs, err := vss.NewVirtualService(watcher, gateways)

	require.NoError(t, err)
	assert.Len(t, vs.Spec.GetHosts(), len(expectedHosts))
	for _, host := range vs.Spec.GetHosts() {
		_, exists := expectedHosts[host]
		assert.True(t, exists)
	}
}

func Test_NewVirtualService_ReturnsError_WhenUnableToCreateHTTPRoute(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()
	watcher.Spec.ServiceInfo = v1beta2.Service{}

	vs, err := vss.NewVirtualService(watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "unable to construct httpRoute from watcher")
}

func Test_NewVirtualService_SetsAHttpRoute(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()

	vs, err := vss.NewVirtualService(watcher, gateways)

	require.NoError(t, err)
	assert.Len(t, vs.Spec.GetHttp(), 1)
}

func Test_NewVirtualService_ReturnsError_WhenFailingToAddOwnerReference(t *testing.T) {
	vss, _ := istio.NewVirtualServiceService(machineryruntime.NewScheme()) // does not provide needed Watcher scheme
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()

	vs, err := vss.NewVirtualService(watcher, gateways)

	require.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrFailedToAddOwnerReference)
}

func Test_NewVirtualService_SetsOwnerReference(t *testing.T) {
	vss := createVirtualServiceService(t)
	watcher := builder.NewWatcherBuilder().Build()
	gateways := createGateways()
	expectedOwnerReference := getOwnerReference(watcher)

	vs, err := vss.NewVirtualService(watcher, gateways)

	require.NoError(t, err)
	ownerReferences := vs.GetOwnerReferences()
	assert.Len(t, ownerReferences, 1)
	assert.Equal(t, expectedOwnerReference, ownerReferences[0])
}

func createVirtualServiceService(t *testing.T) *istio.VirtualServiceService {
	t.Helper()

	vss, _ := istio.NewVirtualServiceService(createScheme(t))
	return vss
}

func createScheme(t *testing.T) *machineryruntime.Scheme {
	t.Helper()

	scheme := machineryruntime.NewScheme()
	err := api.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to setup scheme")
	}

	return scheme
}

const gatewayCount, serverCount = 3, 3

func createGateways() *istioclientapiv1beta1.GatewayList {
	gateways := make([]*istioclientapiv1beta1.Gateway, 0, gatewayCount)
	for gatewayIndex := range gatewayCount {
		gateway := &istioclientapiv1beta1.Gateway{}
		gateway.SetName(fmt.Sprintf("gateway-name-%v", gatewayIndex))
		gateway.SetNamespace(random.Name())

		var servers []*istioapiv1beta1.Server
		for serverIndex := range serverCount {
			server := istioapiv1beta1.Server{}

			var hosts []string
			for hostIndex := range serverCount {
				hosts = append(hosts, fmt.Sprintf("%v-%v-%v.localhost", gatewayIndex, serverIndex, hostIndex))
			}
			server.Hosts = hosts

			servers = append(servers, &server)
		}
		gateway.Spec.Servers = servers

		gateways = append(gateways, gateway)
	}

	return &istioclientapiv1beta1.GatewayList{
		Items: gateways,
	}
}

func getGatewayNamesMap(gateways []*istioclientapiv1beta1.Gateway) map[string]bool {
	gatewayMap := map[string]bool{}

	for _, gateway := range gateways {
		gatewayMap[getGatewayName(gateway)] = true
	}

	return gatewayMap
}

func getHostNamesMap(gateways []*istioclientapiv1beta1.Gateway) map[string]bool {
	hostsMap := map[string]bool{}

	for _, gateway := range gateways {
		for _, server := range gateway.Spec.GetServers() {
			for _, host := range server.GetHosts() {
				hostsMap[host] = true
			}
		}
	}

	return hostsMap
}

func getGatewayName(gateway *istioclientapiv1beta1.Gateway) string {
	return fmt.Sprintf("%v/%v", gateway.Namespace, gateway.Name)
}

func getOwnerReference(watcher *v1beta2.Watcher) apimetav1.OwnerReference {
	return apimetav1.OwnerReference{
		APIVersion: watcher.APIVersion,
		Kind:       watcher.Kind,
		Name:       watcher.GetName(),
		UID:        watcher.GetUID(),
	}
}
