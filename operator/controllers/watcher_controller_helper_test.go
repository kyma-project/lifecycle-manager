package controllers_test

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
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

func verifyVsRoutes(watcherCR *v1alpha1.Watcher, customIstioClient *custom.IstioClient, matcher types.GomegaMatcher) {
	if watcherCR != nil {
		routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, watcherCR)
		Expect(err).ToNot(HaveOccurred())
		Expect(routeReady).To(matcher)
	} else {
		vsDeleted, err := customIstioClient.IsVsDeleted(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(vsDeleted).To(matcher)
	}
}

func isEven(idx int) bool {
	return idx%2 == 0
}

func watcherCRState(watcherObjKey client.ObjectKey) func(g Gomega) v1alpha1.WatcherState {
	return func(g Gomega) v1alpha1.WatcherState {
		watcherCR := &v1alpha1.Watcher{}
		err := controlPlaneClient.Get(ctx, watcherObjKey, watcherCR)
		g.Expect(err).NotTo(HaveOccurred())
		return watcherCR.Status.State
	}
}

func createWatcherCR(moduleName string, statusOnly bool) *v1alpha1.Watcher {
	field := v1alpha1.SpecField
	if statusOnly {
		field = v1alpha1.StatusField
	}
	return &v1alpha1.Watcher{
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
