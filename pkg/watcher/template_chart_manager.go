package watcher

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

// SKRWebhookTemplateChartManager is a SKRWebhookChartManager implementation that renders
// the watcher's helm chart and installs it using a native kube-client.
type SKRWebhookTemplateChartManager struct {
	config  *SkrChartManagerConfig
	kcpAddr string
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
		config:  config,
		kcpAddr: resolvedKcpAddr,
	}, nil
}

func (m *SKRWebhookTemplateChartManager) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)

	// Create CertificateCR which will be used for mTLS connection from SKR to KCP
	// If it already exists, create will do nothing
	certificate, err := NewCertificate(syncContext.ControlPlaneClient, kyma)
	if err != nil {
		logger.Error(err, "Error while creating new Certificate struct")
		return true, err
	}
	if err = certificate.Create(ctx, m.config); err != nil {
		logger.Error(err, "Error while creating new Certificate on KCP")
		return true, err
	}

	// If secret is not created do nothing and check in next reconcile loop
	certSecret, err := certificate.GetSecret(ctx)
	if apierrors.IsNotFound(err) {
		logger.Info("Certificate not ready - Secret not found")
		return true, &CertificateNotReadyError{}
	} else if err != nil {
		logger.Error(err, "Error getting certificate secret")
		return true, err
	}

	chartArgValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient,
		m.config, m.kcpAddr, certSecret)
	if err != nil {
		return true, err
	}
	manifest, err := renderChartToRawManifest(ctx, kyma, m.config.WebhookChartPath, chartArgValues)
	if err != nil {
		return true, err
	}
	resources, err := getRawManifestUnstructuredResources(manifest)
	if err != nil {
		return true, err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource *unstructured.Unstructured) error {
			return clt.Patch(ctx, resource, client.Apply, client.ForceOwnership, skrChartFieldOwner)
		})
	if err != nil {
		return true, fmt.Errorf("failed to apply webhook resources: %w", err)
	}
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook chart",
		"release-name", resolveSKRChartReleaseName(kymaObjKey))
	return true, nil
}

func (m *SKRWebhookTemplateChartManager) Remove(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)

	certificate, err := NewCertificate(syncContext.ControlPlaneClient, kyma)
	if err != nil {
		logger.Error(err, "Error while creating new Certificate struct")
		return err
	}
	if err := certificate.Remove(ctx); err != nil {
		return err
	}

	manifest, err := renderChartToRawManifest(ctx, kyma, m.config.WebhookChartPath, map[string]interface{}{})
	if err != nil {
		return err
	}
	resources, err := getRawManifestUnstructuredResources(manifest)
	if err != nil {
		return err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource *unstructured.Unstructured) error {
			return clt.Delete(ctx, resource)
		})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}
	logger.Info("successfully removed webhook chart",
		"release-name", resolveSKRChartReleaseName(kymaObjKey))
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
