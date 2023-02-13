package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	registrationV1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	podRestartLabelKey     = "operator.kyma-project.io/pod-restart-trigger"
	rawManifestFilePathTpl = "%s/resources.yaml"
	// always true since unsecured watcher setup will no longer be supported.
	tlsEnabled = "true"
)

var (
	ErrExpectedNonNilConfig             = errors.New("expected non nil config")
	ErrExpectedSubjectsNotToBeEmpty     = errors.New("expected subjects to be non empty")
	ErrExpectedNonEmptyPodContainers    = errors.New("expected non empty pod containers")
	ErrPodTemplateMustContainAtLeastOne = errors.New("pod template labels must contain " +
		"at least the deployment selector label")
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

func configureClusterRoleBinding(cfg *unstructuredResourcesConfig, resource *unstructured.Unstructured,
) (*rbacV1.ClusterRoleBinding, error) {
	crb := &rbacV1.ClusterRoleBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, crb); err != nil {
		return nil, err
	}
	if len(crb.Subjects) == 0 {
		return nil, ErrExpectedSubjectsNotToBeEmpty
	}
	serviceAccountSubj := crb.Subjects[0]
	serviceAccountSubj.Namespace = cfg.remoteNs
	crb.Subjects[0] = serviceAccountSubj
	return crb, nil
}

func configureConfigMap(cfg *unstructuredResourcesConfig, resource *unstructured.Unstructured,
) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, configMap); err != nil {
		return nil, err
	}
	configMap.Data = map[string]string{
		"contractVersion":  cfg.contractVersion,
		"kcpAddr":          cfg.kcpAddress,
		"tlsWebhookServer": tlsEnabled,
		"tlsCallback":      tlsEnabled,
	}
	return configMap, nil
}

func configureDeployment(cfg *unstructuredResourcesConfig, obj *unstructured.Unstructured,
) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deployment); err != nil {
		return nil, err
	}

	if deployment.Spec.Template.Labels == nil || len(deployment.Spec.Template.Labels) == 0 {
		return nil, ErrPodTemplateMustContainAtLeastOne
	}
	deployment.Spec.Template.Labels[podRestartLabelKey] = cfg.secretResVer

	// configure resource limits for the webhook server container
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return nil, ErrExpectedNonEmptyPodContainers
	}
	serverContainer := deployment.Spec.Template.Spec.Containers[0]
	cpuResQty, err := resource.ParseQuantity(cfg.cpuResLimit)
	if err != nil {
		return nil, fmt.Errorf("error parsing CPU resource limit: %w", err)
	}
	memResQty, err := resource.ParseQuantity(cfg.memResLimit)
	if err != nil {
		return nil, fmt.Errorf("error parsing memory resource limit: %w", err)
	}
	serverContainer.Resources.Limits = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    cpuResQty,
		corev1.ResourceMemory: memResQty,
	}
	deployment.Spec.Template.Spec.Containers[0] = serverContainer

	return deployment, nil
}

func getGeneratedClientObjects(resourcesConfig *unstructuredResourcesConfig,
	watchableConfigs map[string]WatchableConfig, remoteNs string,
) []client.Object {
	var genClientObjects []client.Object
	webhookCfgObjKey := client.ObjectKey{
		Namespace: remoteNs,
		Name:      SkrResourceName,
	}
	svcObjKey := client.ObjectKey{
		Namespace: remoteNs,
		Name:      SkrResourceName,
	}

	webhookConfig := generateValidatingWebhookConfigFromWatchableConfigs(webhookCfgObjKey, svcObjKey,
		resourcesConfig.caCert, watchableConfigs)
	genClientObjects = append(genClientObjects, webhookConfig)
	secretObjKey := client.ObjectKey{
		Namespace: remoteNs,
		Name:      SkrTLSName,
	}
	skrSecret := createSKRSecret(resourcesConfig, secretObjKey)
	return append(genClientObjects, skrSecret)
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
	secretResVer             string
	cpuResLimit, memResLimit string
	caCert, tlsCert, tlsKey  []byte
	remoteNs                 string
}

func configureUnstructuredObject(cfg *unstructuredResourcesConfig, object *unstructured.Unstructured,
) (client.Object, error) {
	if object.GetAPIVersion() == corev1.SchemeGroupVersion.String() && object.GetKind() == "ConfigMap" {
		return configureConfigMap(cfg, object)
	}
	if object.GetAPIVersion() == appsv1.SchemeGroupVersion.String() && object.GetKind() == "Deployment" {
		return configureDeployment(cfg, object)
	}
	if object.GetAPIVersion() == rbacV1.SchemeGroupVersion.String() && object.GetKind() == "ClusterRoleBinding" {
		return configureClusterRoleBinding(cfg, object)
	}
	return object.DeepCopy(), nil
}

func closeFileAndLogErr(closer io.Closer, logger logr.Logger, path string) {
	err := closer.Close()
	if err != nil {
		logger.V(log.DebugLevel).Info("failed to close raw manifest file", "path", path)
	}
}
