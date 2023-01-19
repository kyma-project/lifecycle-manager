package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SKRWebhookTemplateChartManager is a SKRWebhookChartManager implementation that renders
// the watcher's helm chart and installs it using a native kube-client.
type SKRWebhookTemplateChartManager struct {
	config           *SkrChartManagerConfig
	chartConfigCache *sync.Map
	kcpAddr          string
}

type SkrChartManagerConfig struct {
	// WebhookChartPath represents the path of the webhook chart
	// to be installed on SKR clusters upon reconciling kyma CRs.
	WebhookChartPath       string
	SkrWebhookMemoryLimits string
	SkrWebhookCPULimits    string
	// WatcherLocalTestingEnabled indicates if the chart manager is running in local testing mode
	WatcherLocalTestingEnabled bool
	// GatewayHTTPPortMapping indicates the port used to expose the KCP cluster locally for the watcher callbacks
	GatewayHTTPPortMapping int
}

func NewSKRWebhookTemplateChartManager(kcpRestConfig *rest.Config, config *SkrChartManagerConfig,
) (*SKRWebhookTemplateChartManager, error) {
	resolvedKcpAddr, err := resolveKcpAddr(kcpRestConfig, config)
	if err != nil {
		return nil, err
	}
	return &SKRWebhookTemplateChartManager{
		config:           config,
		chartConfigCache: &sync.Map{},
		kcpAddr:          resolvedKcpAddr,
	}, nil
}

func (m *SKRWebhookTemplateChartManager) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	chartArgValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient, kymaObjKey, m.config, m.kcpAddr)
	if err != nil {
		return true, err
	}
	manifest, err := renderChartToRawManifest(ctx, kymaObjKey, m.config.WebhookChartPath, chartArgValues)
	if err != nil {
		return true, err
	}
	resources, err := getRawManifestUnstructuredResources(manifest)
	if err != nil {
		return true, err
	}
	for _, resource := range resources {
		if err := createOrUpdateResource(ctx, syncContext.RuntimeClient, resource); err != nil {
			return true, err
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
	syncContext := remote.SyncContextFromContext(ctx)
	manifest, err := renderChartToRawManifest(ctx, kymaObjKey, m.config.WebhookChartPath, map[string]interface{}{})
	if err != nil {
		return err
	}
	resources, err := getRawManifestUnstructuredResources(manifest)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if err := syncContext.RuntimeClient.Delete(ctx, resource); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete webhook %s: %w", resource.GetKind(), err)
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
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if errors.Is(err, io.EOF) {
			break
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func createOrUpdateResource(ctx context.Context, skrClient client.Client,
	resource *unstructured.Unstructured,
) error {
	err := skrClient.Create(ctx, resource)
	resourceAlreadyExists := apierrors.IsAlreadyExists(err)
	if err != nil && !resourceAlreadyExists {
		return fmt.Errorf("failed to create webhook %s: %w", resource.GetKind(), err)
	}
	if resourceAlreadyExists {
		resourceObjKey := client.ObjectKeyFromObject(resource)
		oldResource := &unstructured.Unstructured{}
		oldResource.SetGroupVersionKind(resource.GroupVersionKind())
		if err := skrClient.Get(ctx, resourceObjKey, oldResource); err != nil {
			return fmt.Errorf("failed to get webhook %s: %w", resource.GetKind(), err)
		}
		resource.SetResourceVersion(oldResource.GetResourceVersion())
		if err := skrClient.Update(ctx, resource); err != nil {
			return fmt.Errorf("failed to replace webhook %s: %w", resource.GetKind(), err)
		}
		return nil
	}
	return nil
}
