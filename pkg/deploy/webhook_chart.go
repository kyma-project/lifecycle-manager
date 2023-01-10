package deploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	modulelabels "github.com/kyma-project/module-manager/pkg/labels"
	moduletypes "github.com/kyma-project/module-manager/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

type Mode string

const (
	customConfigKey                = "modules"
	WebhookCfgAndDeploymentNameTpl = "%s-webhook"
	IstioSystemNs                  = "istio-system"
	IngressServiceName             = "istio-ingressgateway"
	releaseNameTpl                 = "%s-%s-skr"
	staticWatcherConfigName        = "static-watcher-config-name"
	caCertificateSecretKey         = "ca.crt"
)

var (
	ErrSKRWebhookNotInstalled      = errors.New("skr webhook resources are not installed")
	ErrSKRWebhookWasNotRemoved     = errors.New("installed skr webhook resources were not removed")
	ErrLoadBalancerIPIsNotAssigned = errors.New("load balancer service external ip is not assigned")
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func generateWatchableConfigs(watchers []v1alpha1.Watcher) map[string]WatchableConfig {
	chartCfg := make(map[string]WatchableConfig, 0)
	for _, watcher := range watchers {
		statusOnly := watcher.Spec.Field == v1alpha1.StatusField
		chartCfg[watcher.GetModuleName()] = WatchableConfig{
			Labels:     watcher.Spec.LabelsToWatch,
			StatusOnly: statusOnly,
		}
	}
	return chartCfg
}

func ResolveSKRChartResourceName(resourceNameTpl string, kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(resourceNameTpl, skrChartReleaseName(kymaObjKey))
}

func skrChartReleaseName(kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(releaseNameTpl, kymaObjKey.Namespace, kymaObjKey.Name)
}

func prepareInstallInfo(ctx context.Context, chartPath string, restConfig *rest.Config,
	restClient client.Client, argsVals map[string]interface{}, kymaObjKey client.ObjectKey,
) *moduletypes.InstallInfo {
	return &moduletypes.InstallInfo{
		Ctx: ctx,
		ChartInfo: &moduletypes.ChartInfo{
			ChartPath:   chartPath,
			ReleaseName: skrChartReleaseName(kymaObjKey),
			Flags: moduletypes.ChartFlags{
				SetFlags: argsVals,
			},
		},
		ResourceInfo: &moduletypes.ResourceInfo{
			BaseResource: watcherCachingBaseResource(kymaObjKey),
		},
		ClusterInfo: &moduletypes.ClusterInfo{
			Client: restClient,
			Config: restConfig,
		},
	}
}

func watcherCachingBaseResource(kymaObjKey client.ObjectKey) *unstructured.Unstructured {
	baseRes := &unstructured.Unstructured{}
	baseRes.SetLabels(map[string]string{
		modulelabels.CacheKey: kymaObjKey.String(),
	})
	baseRes.SetNamespace(metav1.NamespaceDefault)
	baseRes.SetName(staticWatcherConfigName)
	return baseRes
}

func ensureWebhookCABundleConsistency(ctx context.Context, skrClient client.Client, kymaObjKey client.ObjectKey,
) error {
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err := skrClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      ResolveSKRChartResourceName(WebhookCfgAndDeploymentNameTpl, kymaObjKey),
	}, webhookConfig)
	if err != nil {
		return fmt.Errorf("error getting webhook config: %w", err)
	}
	tlsSecret := &corev1.Secret{}
	err = skrClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      ResolveSKRChartResourceName("%s-webhook-tls", kymaObjKey),
	}, tlsSecret)
	if err != nil {
		return fmt.Errorf("error getting tls secret: %w", err)
	}
	shouldUpdateWebhookCaBundle := false
	for idx, webhook := range webhookConfig.Webhooks {
		if !bytes.Equal(webhook.ClientConfig.CABundle, tlsSecret.Data[caCertificateSecretKey]) {
			shouldUpdateWebhookCaBundle = true
			webhookConfig.Webhooks[idx].ClientConfig.CABundle = tlsSecret.Data[caCertificateSecretKey]
		}
	}
	if shouldUpdateWebhookCaBundle {
		return skrClient.Update(ctx, webhookConfig, defaultFieldOwner)
	}
	return nil
}

func prettyPrintSetFlags(stringifiedConfig interface{}) string {
	jsonBytes, err := k8syaml.YAMLToJSON([]byte(stringifiedConfig.(string)))
	if err != nil {
		return stringifiedConfig.(string)
	}
	return string(jsonBytes)
}
