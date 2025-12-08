package resources

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	webhookTimeOutInSeconds = 15
	version                 = "v1"
	SkrResourceName         = "skr-webhook"
)

func ResolveWebhookRuleResources(resource string, fieldName v1beta2.FieldName) []string {
	if fieldName == v1beta2.StatusField {
		return []string{fmt.Sprintf("%s/%s", resource, fieldName)}
	}
	return []string{resource}
}

func BuildValidatingWebhookConfigFromWatchers(caCert []byte, watchers []v1beta2.Watcher, remoteNs string,
) *admissionregistrationv1.ValidatingWebhookConfiguration {
	logger := ctrl.Log.WithName("watcher-deprecation")
	webhooks := make([]admissionregistrationv1.ValidatingWebhook, 0)
	for _, watcher := range watchers {
		managerName := watcher.GetManagerName()
		// Log deprecation warning if using managed-by label fallback
		if watcher.Spec.Manager == "" && watcher.Labels != nil && watcher.Labels[shared.ManagedBy] != "" {
			logger.Info("Watcher using deprecated 'managed-by' label. Migrate to spec.manager field",
				"watcher", watcher.Name, "namespace", watcher.Namespace, "manager", managerName)
		}
		webhookName := fmt.Sprintf("%s.%s.%s", watcher.Namespace, watcher.Name, shared.OperatorGroup)
		svcPath := "/validate/" + managerName
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
					Name:      SkrResourceName,
					Namespace: remoteNs,
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
			Name:      SkrResourceName,
			Namespace: remoteNs,
			Labels: map[string]string{
				shared.ManagedBy: shared.ManagedByLabelValue,
			},
		},
		Webhooks: webhooks,
	}
}
