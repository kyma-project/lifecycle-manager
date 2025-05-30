package resources

import (
	"reflect"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TestBuildValidatingWebhookConfigFromWatchers(t *testing.T) {
	ca := []byte("ca-cert")
	remoteNs := "skr-ns"
	watcherManager := shared.OperatorName
	watcher := v1beta2.Watcher{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mod1",
			Namespace: "skr-ns",
			Labels: map[string]string{
				shared.ManagedBy: watcherManager,
			},
		},
		Spec: v1beta2.WatcherSpec{
			ResourceToWatch: v1beta2.WatchableGVR{
				Group:    "operator.kyma-project.io",
				Version:  "*",
				Resource: "kymas",
			},
			Field:         v1beta2.SpecField,
			LabelsToWatch: map[string]string{"foo": "bar"},
		},
	}
	svcPath := "/validate/" + watcherManager
	want := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingWebhookConfiguration",
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      SkrResourceName,
			Namespace: remoteNs,
			Labels: map[string]string{
				shared.ManagedBy: shared.ManagedByLabelValue,
			},
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    "skr-ns.mod1.operator.kyma-project.io",
				ObjectSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				AdmissionReviewVersions: []string{"v1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: ca,
					Service: &admissionregistrationv1.ServiceReference{
						Name:      SkrResourceName,
						Namespace: remoteNs,
						Path:      &svcPath,
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"operator.kyma-project.io"},
							APIVersions: []string{"*"},
							Resources:   []string{"kymas"},
						},
						Operations: []admissionregistrationv1.OperationType{
							"CREATE", "UPDATE", "DELETE",
						},
					},
				},
				SideEffects: func() *admissionregistrationv1.SideEffectClass {
					s := admissionregistrationv1.SideEffectClassNoneOnDryRun
					return &s
				}(),
				TimeoutSeconds: func() *int32 { i := int32(15); return &i }(),
				FailurePolicy:  func() *admissionregistrationv1.FailurePolicyType { f := admissionregistrationv1.Ignore; return &f }(),
			},
		},
	}

	tests := []struct {
		name     string
		caCert   []byte
		watchers []v1beta2.Watcher
		remoteNs string
		want     *admissionregistrationv1.ValidatingWebhookConfiguration
	}{
		{
			name:     "single watcher",
			caCert:   ca,
			watchers: []v1beta2.Watcher{watcher},
			remoteNs: remoteNs,
			want:     want,
		},
		{
			name:     "no watchers returns empty webhooks",
			caCert:   ca,
			watchers: nil,
			remoteNs: remoteNs,
			want: &admissionregistrationv1.ValidatingWebhookConfiguration{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ValidatingWebhookConfiguration",
					APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      SkrResourceName,
					Namespace: remoteNs,
					Labels: map[string]string{
						shared.ManagedBy: shared.ManagedByLabelValue,
					},
				},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildValidatingWebhookConfigFromWatchers(tt.caCert, tt.watchers, tt.remoteNs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildValidatingWebhookConfigFromWatchers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveWebhookRuleResources(t *testing.T) {
	tests := []struct {
		name      string
		resource  string
		fieldName v1beta2.FieldName
		want      []string
	}{
		{
			name:      "spec field returns resource and resource",
			resource:  "kymas",
			fieldName: v1beta2.SpecField,
			want:      []string{"kymas"},
		},
		{
			name:      "status field returns resource/status",
			resource:  "kymas",
			fieldName: v1beta2.StatusField,
			want:      []string{"kymas/status"},
		},
		{
			name:      "unknown field returns resource",
			resource:  "kymas",
			fieldName: "unknown",
			want:      []string{"kymas"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveWebhookRuleResources(tt.resource, tt.fieldName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResolveWebhookRuleResources() = %v, want %v", got, tt.want)
			}
		})
	}
}
