package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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
		kcpAddr: resolvedKcpAddr,
	}, nil
}

func (m *SKRWebhookTemplateChartManager) renderChartToRawManifest(ctx context.Context, kymaObjKey client.ObjectKey,
	syncContext *remote.KymaSynchronizationContext) (string, error) {

	chartArgsValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient, m.config, m.kcpAddr)
	if err != nil {
		return "", err
	}
	chartFS := os.DirFS(m.config.WebhookChartPath)

	chart, err := helm.LoadChart(ctx, chartFS)
	if err != nil {
		return "", nil
	}

	return helm.Template(ctx, helm.TemplateConfig{
		Chart:       chart,
		ReleaseName: skrChartReleaseName(kymaObjKey),
		Namespace:   metav1.NamespaceDefault,
		Values:      chartArgsValues,
	})
}

func (m *SKRWebhookTemplateChartManager) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	manifest, err := m.renderChartToRawManifest(ctx, kymaObjKey, syncContext)
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
			return true, fmt.Errorf("failed to resolve webhook %s: %w", resource.GetKind(), err)
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
			return true, fmt.Errorf("failed to patch webhook %s: %w", resource.GetKind(), err)
		}
	}
	return false, nil
}

func (m *SKRWebhookTemplateChartManager) Remove(ctx context.Context, kyma *v1alpha1.Kyma) error {
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	manifest, err := m.renderChartToRawManifest(ctx, kymaObjKey, syncContext)
	if err != nil {
		return err
	}
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
			return fmt.Errorf("failed to resolve webhook %s: %w", resource.GetKind(), err)
		}
		if err == nil {
			if err := syncContext.RuntimeClient.Delete(ctx, resource); err != nil {
				return fmt.Errorf("failed to delete webhook %s: %w", resource.GetKind(), err)
			}
		}
	}
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
