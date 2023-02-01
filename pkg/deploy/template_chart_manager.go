package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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

// SKRWebhookTemplateChartManager is a SKRWebhookManager implementation that renders
// the watcher's helm chart and installs it using a native kube-client.
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

func (m *SKRWebhookTemplateChartManager) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	remoteNs := resolveRemoteNamespace(kyma)
	chartArgValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient, kymaObjKey, m.config, m.kcpAddr)
	if err != nil {
		return true, err
	}
	manifest, err := renderChartToRawManifest(ctx, kymaObjKey, m.config.WebhookChartPath, chartArgValues)
	if err != nil {
		return true, err
	}
	stringReader := strings.NewReader(manifest)
	resources, err := getRawManifestUnstructuredResources(stringReader, remoteNs)
	if err != nil {
		return true, err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			return clt.Patch(ctx, resource, client.Apply, client.ForceOwnership, skrChartFieldOwner)
		})
	if err != nil {
		return true, fmt.Errorf("failed to apply webhook resources: %w", err)
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
	remoteNs := resolveRemoteNamespace(kyma)
	manifest, err := renderChartToRawManifest(ctx, kymaObjKey, m.config.WebhookChartPath, map[string]interface{}{})
	if err != nil {
		return err
	}
	stringReader := strings.NewReader(manifest)
	resources, err := getRawManifestUnstructuredResources(stringReader, remoteNs)
	if err != nil {
		return err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			return clt.Delete(ctx, resource)
		})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}
	logger.Info("successfully removed webhook chart",
		"release-name", skrChartReleaseName(kymaObjKey))
	return nil
}

func getRawManifestUnstructuredResources(rawManifestReader io.Reader, remoteNs string) ([]client.Object, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(rawManifestReader, defaultBufferSize)
	var resources []client.Object
	for {
		resource := &unstructured.Unstructured{}
		err := decoder.Decode(resource)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if errors.Is(err, io.EOF) {
			break
		}
		resource.SetNamespace(remoteNs)
		resources = append(resources, resource)
	}
	return resources, nil
}
