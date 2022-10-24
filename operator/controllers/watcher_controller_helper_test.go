package controllers_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	. "github.com/onsi/gomega"
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBufferSize = 2048
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

func createWatcherCR(moduleName string, statusOnly bool) *v1alpha1.Watcher {
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
			Name:      fmt.Sprintf("%s-sample", moduleName),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				v1alpha1.ManagedBylabel: moduleName,
			},
		},
		Spec: v1alpha1.WatcherSpec{
			ServiceInfo: v1alpha1.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", moduleName),
				Namespace: metav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", moduleName): "true",
			},
			Field: field,
		},
	}
}

func createKymaCR(kymaName string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1alpha1.KymaKind),
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.KymaSpec{
			Channel: v1alpha1.ChannelStable,
			Modules: []v1alpha1.Module{
				{
					Name: "sample-skr-module",
				},
				{
					Name: "sample-kcp-module",
				},
			},
			Sync: v1alpha1.Sync{
				Enabled:  false,
				Strategy: v1alpha1.SyncStrategyLocalClient,
			},
		},
	}
}

func isCrDeletionFinished(watcherObjKeys ...client.ObjectKey) func(g Gomega) bool {
	if len(watcherObjKeys) > 1 {
		return nil
	}
	if len(watcherObjKeys) == 0 {
		return func(g Gomega) bool {
			watchers := &v1alpha1.WatcherList{}
			err := controlPlaneClient.List(ctx, watchers)
			return err == nil && len(watchers.Items) == 0
		}
	}
	return func(g Gomega) bool {
		err := controlPlaneClient.Get(ctx, watcherObjKeys[0], &v1alpha1.Watcher{})
		return apierrors.IsNotFound(err)
	}
}

func isCrVsConfigured(ctx context.Context, customIstioClient *custom.IstioClient, obj *v1alpha1.Watcher,
) func(g Gomega) bool {
	return func(g Gomega) bool {
		routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, obj)
		return err == nil && routeReady
	}
}

func isVsRemoved(ctx context.Context, customIstioClient *custom.IstioClient) func(g Gomega) bool {
	return func(g Gomega) bool {
		vsDeleted, err := customIstioClient.IsVsDeleted(ctx)
		return err == nil && vsDeleted
	}
}
