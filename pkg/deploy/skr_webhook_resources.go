package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"go.uber.org/zap"

	registrationV1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	bindingSubjectsKey = "subjects"
	stringMapDataKey   = "data"
	podRestartLabelKey = "operator.kyma-project.io/pod-restart-trigger"
)

var (
	ErrExpectedNonNilConfig         = errors.New("expected non nil config")
	ErrFailedToConvertSubjectToMap  = errors.New("failed to convert subject to a map")
	ErrExpectedSubjectsNotToBeEmpty = errors.New("expected subjects to be non empty")
	ErrCouldNotFindSubjectsField    = fmt.Errorf("could not find %s field", bindingSubjectsKey)
)

func createSKRSecret(cfg *unstructuredResourcesConfig, secretObjKey client.ObjectKey,
) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretObjKey.Name,
			Namespace: secretObjKey.Namespace,
		},
		Immutable: nil,
		Data: map[string][]byte{
			caCertKey:        cfg.caCert,
			tlsCertKey:       cfg.tlsCert,
			tlsPrivateKeyKey: cfg.tlsKey,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func generateValidatingWebhookConfigFromWatchableConfigs(webhookObjKey, svcObjKey client.ObjectKey, caCert []byte,
	watchableConfigs map[string]WatchableConfig,
) *registrationV1.ValidatingWebhookConfiguration {
	webhooks := make([]registrationV1.ValidatingWebhook, 0)
	for moduleName, watchableCfg := range watchableConfigs {
		svcPath := fmt.Sprintf("/validate/%s", moduleName)
		watchableResources := allResourcesWebhookRule
		if watchableCfg.StatusOnly {
			watchableResources = statusSubResourceWebhookRule
		}
		sideEffects := registrationV1.SideEffectClassNoneOnDryRun
		failurePolicy := registrationV1.Ignore
		timeout := new(int32)
		*timeout = webhookTimeOutInSeconds
		webhook := registrationV1.ValidatingWebhook{
			Name:                    fmt.Sprintf("%s.operator.kyma-project.io", moduleName),
			ObjectSelector:          &metav1.LabelSelector{MatchLabels: watchableCfg.Labels},
			AdmissionReviewVersions: []string{version},
			ClientConfig: registrationV1.WebhookClientConfig{
				CABundle: caCert,
				Service: &registrationV1.ServiceReference{
					Name:      svcObjKey.Name,
					Namespace: svcObjKey.Namespace,
					Path:      &svcPath,
				},
			},
			Rules: []registrationV1.RuleWithOperations{
				{
					Rule: registrationV1.Rule{
						APIGroups:   []string{v1alpha1.GroupVersion.Group},
						APIVersions: []string{"*"},
						Resources:   []string{watchableResources},
					},
					Operations: []registrationV1.OperationType{
						"CREATE", "UPDATE", "DELETE",
					},
				},
			},
			SideEffects:    &sideEffects,
			TimeoutSeconds: timeout,
			FailurePolicy:  &failurePolicy,
		}
		webhooks = append(webhooks, webhook)
	}
	return &registrationV1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingWebhookConfiguration",
			APIVersion: registrationV1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookObjKey.Name,
			Namespace: webhookObjKey.Namespace,
		},
		Webhooks: webhooks,
	}
}

func configureClusterRoleBinding(cfg *unstructuredResourcesConfig, binding *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	subjects, found, err := unstructured.NestedSlice(binding.UnstructuredContent(), bindingSubjectsKey)
	if !found {
		return nil, ErrCouldNotFindSubjectsField
	}
	if err != nil {
		return nil, err
	}
	if len(subjects) == 0 {
		return nil, ErrExpectedSubjectsNotToBeEmpty
	}
	serviceAccountSubj, ok := subjects[0].(map[string]interface{})
	if !ok {
		return nil, ErrFailedToConvertSubjectToMap
	}
	serviceAccountSubj["namespace"] = cfg.remoteNs
	subjects[0] = serviceAccountSubj
	if err := unstructured.SetNestedSlice(binding.Object, subjects, bindingSubjectsKey); err != nil {
		return nil, err
	}
	return binding, nil
}

func configureConfigMap(cfg *unstructuredResourcesConfig, configMap *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	configMapData := map[string]string{
		"version":  cfg.contractVersion,
		"kcpAddr":          cfg.kcpAddress,
		"tlsWebhookServer": cfg.tlsWebhookServer,
		"tlsCallback":      cfg.tlsCallback,
	}
	if err := unstructured.SetNestedStringMap(configMap.Object, configMapData, stringMapDataKey); err != nil {
		return nil, err
	}
	return configMap, nil
}

func configureDeploymentLabel(cfg *unstructuredResourcesConfig, deployment *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	err := unstructured.SetNestedField(deployment.Object, cfg.secretResVer, "spec", "template", "metadata",
		"labels", podRestartLabelKey)
	if err != nil {
		return nil, fmt.Errorf("failed to update pod template labels: %w", err)
	}

	podVolumes, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "volumes")
	if !found {
		return nil, errors.New("pod volumes not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pod volumes: %w", err)
	}
	if len(podVolumes) == 0 {
		return nil, ErrExpectedSubjectsNotToBeEmpty
	}
	sslVolume, ok := podVolumes[0].(map[string]interface{})
	if !ok {
		return nil, ErrFailedToConvertSubjectToMap
	}

	err = unstructured.SetNestedField(sslVolume, cfg.secretName, "secret", "secretName")
	if err != nil {
		return nil, fmt.Errorf("failed to set secret name: %w", err)
	}
	podVolumes[0] = sslVolume
	err = unstructured.SetNestedSlice(deployment.Object, podVolumes, "spec", "template", "spec", "volumes")
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment volumes: %w", err)
	}
	return deployment, nil
}

func getSKRClientObjectsForInstall(ctx context.Context, kcpClient client.Client, kymaObjKey client.ObjectKey,
	remoteNs, kcpAddr, chartPath string, logger logr.Logger,
) ([]client.Object, error) {
	var skrClientObjects []client.Object
	resourcesConfig, err := getUnstructuredResourcesConfig(ctx, kcpClient, kymaObjKey, remoteNs, kcpAddr)
	if err != nil {
		return nil, err
	}
	resources, err := getRawManifestClientObjects(resourcesConfig, remoteNs, chartPath, logger)
	if err != nil {
		return nil, err
	}
	skrClientObjects = append(skrClientObjects, resources...)
	webhookCfgObjKey := client.ObjectKey{
		Namespace: remoteNs,
		Name:      ResolveSKRChartResourceName(WebhookCfgAndDeploymentNameTpl, kymaObjKey),
	}
	svcObjKey := client.ObjectKey{
		Namespace: remoteNs,
		Name:      ResolveSKRChartResourceName(WebhookSvcNameTpl, kymaObjKey),
	}

	watchableConfigs, err := getWatchableConfigs(ctx, kcpClient)
	if err != nil {
		return nil, err
	}
	webhookConfig := generateValidatingWebhookConfigFromWatchableConfigs(webhookCfgObjKey, svcObjKey,
		resourcesConfig.caCert, watchableConfigs)
	skrClientObjects = append(skrClientObjects, webhookConfig)
	secretObjKey := client.ObjectKey{
		Namespace: remoteNs,
		Name:      ResolveSKRChartResourceName(WebhookTLSCfgNameTpl, kymaObjKey),
	}
	skrSecret := createSKRSecret(resourcesConfig, secretObjKey)
	return append(skrClientObjects, skrSecret), nil
}

func getWatchableConfigs(ctx context.Context, kcpClient client.Client) (map[string]WatchableConfig, error) {
	watchableConfigs := map[string]WatchableConfig{}
	watcherList := &v1alpha1.WatcherList{}
	if err := kcpClient.List(ctx, watcherList); err != nil {
		return nil, fmt.Errorf("error listing watcher CRs: %w", err)
	}

	watchers := watcherList.Items
	if len(watchers) != 0 {
		watchableConfigs = generateWatchableConfigs(watchers)
	}
	return watchableConfigs, nil
}

type unstructuredResourcesConfig struct {
	contractVersion          string
	kcpAddress               string
	tlsWebhookServer         string
	tlsCallback              string
	secretName, secretResVer string
	caCert, tlsCert, tlsKey  []byte
	remoteNs                 string
}

func configureUnstructuredResource(cfg *unstructuredResourcesConfig, resource *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	if resource.GetAPIVersion() == corev1.SchemeGroupVersion.String() && resource.GetKind() == "ConfigMap" {
		return configureConfigMap(cfg, resource)
	}
	if resource.GetAPIVersion() == appsv1.SchemeGroupVersion.String() && resource.GetKind() == "Deployment" {
		return configureDeploymentLabel(cfg, resource)
	}
	if resource.GetAPIVersion() == rbacV1.SchemeGroupVersion.String() && resource.GetKind() == "ClusterRoleBinding" {
		return configureClusterRoleBinding(cfg, resource)
	}
	return resource, nil
}

func getRawManifestClientObjects(cfg *unstructuredResourcesConfig, remoteNs, chartPath string, logger logr.Logger,
) ([]client.Object, error) {
	if cfg == nil {
		return nil, ErrExpectedNonNilConfig
	}
	manifestFilePath := fmt.Sprintf("%s/raw/skr-webhook-resources.yaml", chartPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return nil, err
	}
	defer func(closer io.Closer) {
		err := closer.Close()
		if err != nil {
			logger.V(int(zap.DebugLevel)).Info("failed to close raw manifest file", "path",
				manifestFilePath)
		}
	}(rawManifestFile)
	decoder := yaml.NewYAMLOrJSONDecoder(rawManifestFile, defaultBufferSize)
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
		configuredResource, err := configureUnstructuredResource(cfg, resource)
		if err != nil {
			return nil, fmt.Errorf("failed to configure %s resource: %w", resource.GetKind(), err)
		}
		resources = append(resources, configuredResource)
	}
	return resources, nil
}

func getUnstructuredResourcesConfig(ctx context.Context, kcpClient client.Client, kymaObjKey client.ObjectKey,
	remoteNs, kcpAddr string,
) (*unstructuredResourcesConfig, error) {
	tlsSecret := &corev1.Secret{}
	secretObjKey := client.ObjectKey{
		Namespace: kymaObjKey.Namespace,
		Name:      ResolveSKRChartResourceName(WebhookTLSCfgNameTpl, kymaObjKey),
	}

	if err := kcpClient.Get(ctx, secretObjKey, tlsSecret); err != nil {
		return nil, fmt.Errorf("error fetching TLS secret: %w", err)
	}

	return &unstructuredResourcesConfig{
		contractVersion:  version,
		kcpAddress:       kcpAddr,
		tlsWebhookServer: "true",
		tlsCallback:      "true",
		secretName:       tlsSecret.Name,
		secretResVer:     tlsSecret.ResourceVersion,
		caCert:           tlsSecret.Data[caCertKey],
		tlsCert:          tlsSecret.Data[tlsCertKey],
		tlsKey:           tlsSecret.Data[tlsPrivateKeyKey],
		remoteNs:         remoteNs,
	}, nil
}
