package deploy

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"io"
	"os"
	"reflect"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"sync"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/slok/go-helm-template/helm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBufferSize = 2048
	defaultFieldOwner = client.FieldOwner("lifecycle-manager")
)

type SKRWebhookTemplateChartManager struct {
	config  *SkrChartManagerConfig
	cache   *sync.Map
	kcpAddr string
}

func NewSKRWebhookTemplateChartManager(kcpRestConfig *rest.Config, config *SkrChartManagerConfig,
) (*SKRWebhookTemplateChartManager, error) {
	resolvedKcpAddr, err := resolveKcpAddr(kcpRestConfig, config)
	if err != nil {
		return nil, err
	}
	return &SKRWebhookTemplateChartManager{
		config:  config,
		cache:   &sync.Map{},
		kcpAddr: resolvedKcpAddr,
	}, nil
}

func (m *SKRWebhookTemplateChartManager) renderChartToRawManifest(ctx context.Context, kymaObjKey client.ObjectKey,
	chartArgValues map[string]interface{},
) (string, error) {
	chartFS := os.DirFS(m.config.WebhookChartPath)
	chart, err := helm.LoadChart(ctx, chartFS)
	if err != nil {
		return "", nil
	}
	return helm.Template(ctx, helm.TemplateConfig{
		Chart:       chart,
		ReleaseName: skrChartReleaseName(kymaObjKey),
		Namespace:   metav1.NamespaceDefault,
		Values:      chartArgValues,
	})
}

func (m *SKRWebhookTemplateChartManager) cachedConfig(kymaObjKey client.ObjectKey,
	chartArgValues map[string]interface{}) bool {
	cachedArgValues, ok := m.cache.Load(kymaObjKey)
	if !ok {
		m.cache.Store(kymaObjKey, chartArgValues)
		return false
	}
	if !reflect.DeepEqual(chartArgValues, cachedArgValues) {
		m.cache.Store(kymaObjKey, chartArgValues)
		return false
	}
	return true
}

func (m *SKRWebhookTemplateChartManager) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	chartArgValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient, m.config, m.kcpAddr)
	if err != nil {
		return true, err
	}
	if m.cachedConfig(kymaObjKey, chartArgValues) {
		logger.V(int(zap.DebugLevel)).Info("webhook chart config is already installed",
			"release-name", skrChartReleaseName(kymaObjKey))
		return false, nil
	}
	manifest, err := m.renderChartToRawManifest(ctx, kymaObjKey, chartArgValues)
	if err != nil {
		return true, err
	}
	resources, err := getRawManifestUnstructuredResources(manifest)
	if err != nil {
		return true, err
	}
	for _, resource := range resources {
		resourceObjKey := client.ObjectKeyFromObject(resource)
		oldResource := &unstructured.Unstructured{}
		oldResource.SetGroupVersionKind(resource.GroupVersionKind())
		err := syncContext.RuntimeClient.Get(ctx, resourceObjKey, oldResource)
		if err != nil && !apierrors.IsNotFound(err) {
			return true, fmt.Errorf("failed to get webhook %s: %w", resource.GetKind(), err)
		}
		if apierrors.IsNotFound(err) {
			if err := syncContext.RuntimeClient.Create(ctx, resource, defaultFieldOwner); err != nil {
				return true, fmt.Errorf("failed to create webhook %s: %w", resource.GetKind(), err)
			}
		}
		// completely replace old resource with new resource
		resource.SetResourceVersion(oldResource.GetResourceVersion())
		err = syncContext.RuntimeClient.Update(ctx, resource, defaultFieldOwner)
		if err != nil {
			return true, fmt.Errorf("failed to replace webhook %s: %w", resource.GetKind(), err)
		}
	}
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook chart",
		"release-name", skrChartReleaseName(kymaObjKey))
	return false, nil
}

func (m *SKRWebhookTemplateChartManager) Remove(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	// remove cached configs for this kyma
	m.cache.Delete(kymaObjKey)
	syncContext := remote.SyncContextFromContext(ctx)
	chartArgValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient, m.config, m.kcpAddr)
	if err != nil {
		return err
	}
	manifest, err := m.renderChartToRawManifest(ctx, kymaObjKey, chartArgValues)
	if err != nil {
		return err
	}
	logger.V(int(zap.DebugLevel)).Info("following yaml manifest will be removed",
		"manifest", manifest)
	resources, err := getRawManifestUnstructuredResources(manifest)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		resourceObjKey := client.ObjectKeyFromObject(resource)
		oldResource := &unstructured.Unstructured{}
		oldResource.SetGroupVersionKind(resource.GroupVersionKind())
		err := syncContext.RuntimeClient.Get(ctx, resourceObjKey, oldResource)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get webhook %s: %w", resource.GetKind(), err)
		}
		if err == nil {
			if err := syncContext.RuntimeClient.Delete(ctx, resource); err != nil {
				return fmt.Errorf("failed to delete webhook %s: %w", resource.GetKind(), err)
			}
		}
	}
	logger.Info("successfully removed webhook chart",
		"release-name", skrChartReleaseName(kymaObjKey))
	return nil
}

func getRawManifestUnstructuredResources(rawManifest string) ([]*unstructured.Unstructured, error) {
	stringReader := strings.NewReader(rawManifest)
	k8syaml.NewYAMLOrJSONDecoder(stringReader, defaultBufferSize)
	decoder := k8syaml.NewYAMLOrJSONDecoder(stringReader, defaultBufferSize)
	var resources []*unstructured.Unstructured
	for {
		resource := &unstructured.Unstructured{}
		err := decoder.Decode(resource)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if err == io.EOF {
			break
		}
		resources = append(resources, resource)
	}
	return resources, nil
}
