package withwatcher_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBufferSize = 2048
	crToDeleteIdx     = 2
)

//nolint:gochecknoglobals
var centralComponents = []string{"lifecycle-manager", "module-manager", "compass"}

func deserializeIstioResources() ([]*unstructured.Unstructured, error) {
	var istioResourcesList []*unstructured.Unstructured

	file, err := os.Open(istioResourcesFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
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
			Name:      fmt.Sprintf("%s-sample", managerInstanceName),
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
		},
	}
}

func listTestWatcherCrs(kcpClient client.Client) []*v1alpha1.Watcher {
	watchers := make([]*v1alpha1.Watcher, 0)
	for _, component := range centralComponents {
		watcherCR := &v1alpha1.Watcher{}
		err := kcpClient.Get(suiteCtx, client.ObjectKey{
			Name:      fmt.Sprintf("%s-sample", component),
			Namespace: metav1.NamespaceDefault,
		}, watcherCR)
		if err != nil {
			continue
		}

		watchers = append(watchers, watcherCR)
	}
	return watchers
}

func isCrDeletionFinished(watcherObjKeys ...client.ObjectKey) func(g Gomega) bool {
	if len(watcherObjKeys) > 1 {
		return nil
	}
	if len(watcherObjKeys) == 0 {
		return func(g Gomega) bool {
			watchers := listTestWatcherCrs(controlPlaneClient)
			return len(watchers) == 0
		}
	}
	return func(g Gomega) bool {
		err := controlPlaneClient.Get(suiteCtx, watcherObjKeys[0], &v1alpha1.Watcher{})
		return apierrors.IsNotFound(err)
	}
}

func isCrVsConfigured(ctx context.Context, customIstioClient *istio.Client, obj *v1alpha1.Watcher,
) func(g Gomega) bool {
	return func(g Gomega) bool {
		routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, obj)
		return err == nil && routeReady
	}
}

func isVsRemoved(ctx context.Context, customIstioClient *istio.Client) func(g Gomega) bool {
	return func(g Gomega) bool {
		vsDeleted, err := customIstioClient.IsVsDeleted(ctx)
		return err == nil && vsDeleted
	}
}
