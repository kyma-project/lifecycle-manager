package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apirbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	podRestartLabelKey      = shared.OperatorGroup + shared.Separator + "pod-restart-trigger"
	kcpAddressEnvName       = "KCP_ADDR"
	watcherBaseImageAddress = "europe-docker.pkg.dev/kyma-project/prod/"
	SkrTLSName              = "skr-webhook-tls"
	SkrResourceName         = "skr-webhook"
	skrChartFieldOwner      = client.FieldOwner(shared.OperatorName)
	version                 = "v1"
	webhookTimeOutInSeconds = 15
)

var (
	errExpectedSubjectsNotToBeEmpty     = errors.New("expected subjects to be non empty")
	errExpectedNonEmptyPodContainers    = errors.New("expected non empty pod containers")
	errPodTemplateMustContainAtLeastOne = errors.New("pod template labels must contain " +
		"at least the deployment selector label")
)

func createSKRSecret(cfg *unstructuredResourcesConfig, secretObjKey client.ObjectKey,
) *apicorev1.Secret {
	return &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      secretObjKey.Name,
			Namespace: secretObjKey.Namespace,
		},
		Immutable: nil,
		Data: map[string][]byte{
			caCertKey:        cfg.caCert,
			tlsCertKey:       cfg.tlsCert,
			tlsPrivateKeyKey: cfg.tlsKey,
		},
		Type: apicorev1.SecretTypeOpaque,
	}
}

func ResolveWebhookRuleResources(resource string, fieldName v1beta2.FieldName) []string {
	if fieldName == v1beta2.StatusField {
		return []string{fmt.Sprintf("%s/%s", resource, fieldName)}
	}
	return []string{resource}
}

func generateValidatingWebhookConfigFromWatchers(webhookObjKey,
	svcObjKey client.ObjectKey, caCert []byte, watchers []v1beta2.Watcher,
) *admissionregistrationv1.ValidatingWebhookConfiguration {
	webhooks := make([]admissionregistrationv1.ValidatingWebhook, 0)
	for _, watcher := range watchers {
		moduleName := watcher.GetModuleName()
		webhookName := fmt.Sprintf("%s.%s.%s", watcher.Namespace, watcher.Name, shared.OperatorGroup)
		svcPath := fmt.Sprintf("/validate/%s", moduleName)
		watchableResources := ResolveWebhookRuleResources(watcher.Spec.ResourceToWatch.Resource, watcher.Spec.Field)
		sideEffects := admissionregistrationv1.SideEffectClassNoneOnDryRun
		failurePolicy := admissionregistrationv1.Ignore
		timeout := new(int32)
		*timeout = webhookTimeOutInSeconds
		webhook := admissionregistrationv1.ValidatingWebhook{
			Name:                    webhookName,
			ObjectSelector:          &apimetav1.LabelSelector{MatchLabels: watcher.Spec.LabelsToWatch},
			AdmissionReviewVersions: []string{version},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caCert,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      svcObjKey.Name,
					Namespace: svcObjKey.Namespace,
					Path:      &svcPath,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{watcher.Spec.ResourceToWatch.Group},
						APIVersions: []string{watcher.Spec.ResourceToWatch.Version},
						Resources:   watchableResources,
					},
					Operations: []admissionregistrationv1.OperationType{
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
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "ValidatingWebhookConfiguration",
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      webhookObjKey.Name,
			Namespace: webhookObjKey.Namespace,
		},
		Webhooks: webhooks,
	}
}

var errConvertUnstruct = errors.New("failed to convert deployment to unstructured")

func configureClusterRoleBinding(cfg *unstructuredResourcesConfig, resource *unstructured.Unstructured,
) (*apirbacv1.ClusterRoleBinding, error) {
	crb := &apirbacv1.ClusterRoleBinding{}
	if err := machineryruntime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, crb); err != nil {
		return nil, fmt.Errorf("%w: %w", errConvertUnstruct, err)
	}
	if len(crb.Subjects) == 0 {
		return nil, errExpectedSubjectsNotToBeEmpty
	}
	serviceAccountSubj := crb.Subjects[0]
	serviceAccountSubj.Namespace = cfg.remoteNs
	crb.Subjects[0] = serviceAccountSubj
	return crb, nil
}

func configureDeployment(cfg *unstructuredResourcesConfig, obj *unstructured.Unstructured,
) (*apiappsv1.Deployment, error) {
	deployment := &apiappsv1.Deployment{}
	if err := machineryruntime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deployment); err != nil {
		return nil, fmt.Errorf("%w: %w", errConvertUnstruct, err)
	}
	if deployment.Spec.Template.Labels == nil || len(deployment.Spec.Template.Labels) == 0 {
		return nil, errPodTemplateMustContainAtLeastOne
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return nil, errExpectedNonEmptyPodContainers
	}
	deployment.Spec.Template.Labels[podRestartLabelKey] = cfg.secretResVer

	serverContainer := deployment.Spec.Template.Spec.Containers[0]
	if cfg.skrWatcherImage != "" {
		serverContainer.Image = fmt.Sprintf("%s%s", watcherBaseImageAddress, cfg.skrWatcherImage)
	}

	for i := 0; i < len(serverContainer.Env); i++ {
		if serverContainer.Env[i].Name == kcpAddressEnvName {
			serverContainer.Env[i].Value = cfg.kcpAddress
		}
	}

	// configure resource limits for the webhook server container
	cpuResQty, err := resource.ParseQuantity(cfg.cpuResLimit)
	if err != nil {
		return nil, fmt.Errorf("error parsing CPU resource limit: %w", err)
	}
	memResQty, err := resource.ParseQuantity(cfg.memResLimit)
	if err != nil {
		return nil, fmt.Errorf("error parsing memory resource limit: %w", err)
	}
	serverContainer.Resources.Limits = map[apicorev1.ResourceName]resource.Quantity{
		apicorev1.ResourceCPU:    cpuResQty,
		apicorev1.ResourceMemory: memResQty,
	}
	deployment.Spec.Template.Spec.Containers[0] = serverContainer

	return deployment, nil
}

func getGeneratedClientObjects(resourcesConfig *unstructuredResourcesConfig,
	watchers []v1beta2.Watcher, remoteNs string,
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

func getWatchers(ctx context.Context, kcpClient client.Client) ([]v1beta2.Watcher, error) {
	watcherList := &v1beta2.WatcherList{}
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
	if object.GetAPIVersion() == apiappsv1.SchemeGroupVersion.String() && object.GetKind() == "Deployment" {
		return configureDeployment(cfg, object)
	}
	if object.GetAPIVersion() == apirbacv1.SchemeGroupVersion.String() && object.GetKind() == "ClusterRoleBinding" {
		return configureClusterRoleBinding(cfg, object)
	}
	return object.DeepCopy(), nil
}

func closeFileAndLogErr(ctx context.Context, closer io.Closer, path string) {
	logger := logf.FromContext(ctx)
	err := closer.Close()
	if err != nil {
		logger.V(log.DebugLevel).Info("failed to close raw manifest file", "path", path)
	}
}
