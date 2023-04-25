package watcher

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"io"

	registrationV1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
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

func ResolveWebhookRuleResources(resource string, fieldName v1beta1.FieldName) []string {
	if fieldName == v1beta1.StatusField {
		return []string{fmt.Sprintf("%s/%s", resource, fieldName)}
	}
	return []string{resource}
}

func generateValidatingWebhookConfigFromWatchers(webhookObjKey,
	svcObjKey client.ObjectKey, caCert []byte, watchers []v1beta1.Watcher,
) *registrationV1.ValidatingWebhookConfiguration {
	webhooks := make([]registrationV1.ValidatingWebhook, 0)
	for _, watcher := range watchers {
		moduleName := watcher.GetModuleName()
		webhookName := fmt.Sprintf("%s.%s.operator.kyma-project.io", watcher.Namespace, watcher.Name)
		svcPath := fmt.Sprintf("/validate/%s", moduleName)
		watchableResources := ResolveWebhookRuleResources(watcher.Spec.ResourceToWatch.Resource, watcher.Spec.Field)
		sideEffects := registrationV1.SideEffectClassNoneOnDryRun
		failurePolicy := registrationV1.Ignore
		timeout := new(int32)
		*timeout = webhookTimeOutInSeconds
		webhook := registrationV1.ValidatingWebhook{
			Name:                    webhookName,
			ObjectSelector:          &metav1.LabelSelector{MatchLabels: watcher.Spec.LabelsToWatch},
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
						APIGroups:   []string{watcher.Spec.ResourceToWatch.Group},
						APIVersions: []string{watcher.Spec.ResourceToWatch.Version},
						Resources:   watchableResources,
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
	if cfg.skrWatcherImage != "" {
		serverContainer.Image = cfg.skrWatcherImage
	}
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
	watchers []v1beta1.Watcher, remoteNs string,
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

	webhookConfig := generateValidatingWebhookConfigFromWatchers(webhookCfgObjKey, svcObjKey,
		resourcesConfig.caCert, watchers)
	genClientObjects = append(genClientObjects, webhookConfig)
	secretObjKey := client.ObjectKey{
		Namespace: remoteNs,
		Name:      SkrTLSName,
	}
	skrSecret := createSKRSecret(resourcesConfig, secretObjKey)
	return append(genClientObjects, skrSecret)
}

func getWatchers(ctx context.Context, kcpClient client.Client) ([]v1beta1.Watcher, error) {
	watcherList := &v1beta1.WatcherList{}
	if err := kcpClient.List(ctx, watcherList); err != nil {
		return nil, fmt.Errorf("error listing watcher CRs: %w", err)
	}

	return watcherList.Items, nil
}

type unstructuredResourcesConfig struct {
	contractVersion          string
	kcpAddress               string
	secretResVer             string
	cpuResLimit, memResLimit string
	skrWatcherImage          string
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
