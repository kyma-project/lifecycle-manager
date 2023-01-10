package withwatcher_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	istioclientapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBufferSize    = 2048
	componentToBeRemoved = "compass"
	componentToBeUpdated = "lifecycle-manager"
)

//nolint:gochecknoglobals
var (
	centralComponents                     = []string{componentToBeUpdated, "module-manager", componentToBeRemoved}
	errRouteNotExists                     = errors.New("http route is not exists")
	errVirtualServiceNotRemoved           = errors.New("virtual service not removed")
	errWatcherNotRemoved                  = errors.New("watcher CR not removed")
	errVirtualServiceHostsNotMatchGateway = errors.New("virtual service hosts not match with gateway")
)

func deserializeIstioResources() ([]*unstructured.Unstructured, error) {
	var istioResourcesList []*unstructured.Unstructured

	file, err := os.Open(istioResourcesFilePath)
	if err != nil {
		return nil, err
	}
	defer func(file io.ReadCloser) {
		err := file.Close()
		if err != nil {
			logger.Error(err, "failed to close test resources", "path", istioResourcesFilePath)
		}
	}(file)
	decoder := yaml.NewYAMLOrJSONDecoder(file, defaultBufferSize)
	for {
		istioResource := &unstructured.Unstructured{}
		err = decoder.Decode(istioResource)
		if err == nil {
			istioResourcesList = append(istioResourcesList, istioResource)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return istioResourcesList, nil
}

func isEven(idx int) bool {
	return idx%2 == 0
}

func createWatcherCR(managerInstanceName string, statusOnly bool) *v1alpha1.Watcher {
	field := v1alpha1.SpecField
	if statusOnly {
		field = v1alpha1.StatusField
	}
	return &v1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1alpha1.WatcherKind),
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerInstanceName,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				v1alpha1.ManagedBy: managerInstanceName,
			},
		},
		Spec: v1alpha1.WatcherSpec{
			ServiceInfo: v1alpha1.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", managerInstanceName),
				Namespace: metav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", managerInstanceName): "true",
			},
			Field: field,
			Gateway: v1alpha1.GatewayConfig{
				LabelSelector: v1alpha1.DefaultIstioGatewaySelector(),
			},
		},
	}
}

func getWatcher(name string) (v1alpha1.Watcher, error) {
	watcher := v1alpha1.Watcher{}
	err := controlPlaneClient.Get(suiteCtx,
		client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault},
		&watcher)
	return watcher, err
}

func isVirtualServiceHTTPRouteConfigured(ctx context.Context, customIstioClient *istio.Client, obj *v1alpha1.Watcher,
) error {
	routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, obj)
	if !routeReady {
		return errRouteNotExists
	}
	return err
}

func isVirtualServiceHostsConfigured(ctx context.Context,
	istioClient *istio.Client,
	gateway *istioclientapi.Gateway,
) error {
	virtualService, err := istioClient.GetVirtualService(ctx)
	if err != nil {
		return err
	}
	if !contains(virtualService.Spec.Hosts, gateway.Spec.Servers[0].Hosts[0]) {
		return errVirtualServiceHostsNotMatchGateway
	}
	return nil
}

func contains(source []string, target string) bool {
	for _, item := range source {
		if item == target {
			return true
		}
	}
	return false
}

func isVirtualServiceRemoved(ctx context.Context, customIstioClient *istio.Client) error {
	vsDeleted, err := customIstioClient.IsVirtualServiceDeleted(ctx)
	if !vsDeleted {
		return errVirtualServiceNotRemoved
	}
	return err
}
