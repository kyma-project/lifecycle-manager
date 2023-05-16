package testutils

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KymaBuilder struct {
	kyma *v1beta2.Kyma
}

// NewKymaBuilder returns a KymaBuilder with v1beta2.Kyma initialized defaults
func NewKymaBuilder() KymaBuilder {
	return KymaBuilder{
		kyma: &v1beta2.Kyma{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta2.GroupVersion.String(),
				Kind:       string(v1beta2.KymaKind),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      randomName(),
				Namespace: metav1.NamespaceDefault},
			Spec:   v1beta2.KymaSpec{},
			Status: v1beta2.KymaStatus{},
		},
	}
}

// WithName sets v1beta2.Kyma.ObjectMeta.Name
func (kb KymaBuilder) WithName(name string) KymaBuilder {
	kb.kyma.ObjectMeta.Name = name
	return kb
}

// WithAnnotation adds an annotation to v1beta2.Kyma.ObjectMeta.Annotation
func (kb KymaBuilder) WithAnnotation(key string, value string) KymaBuilder {
	if kb.kyma.Annotations == nil {
		kb.kyma.Annotations = map[string]string{}
	}
	kb.kyma.Annotations[key] = value
	return kb
}

// WithLabel adds a label to v1beta2.Kyma.ObjectMeta.Labels
func (kb KymaBuilder) WithLabel(key string, value string) KymaBuilder {
	if kb.kyma.Labels == nil {
		kb.kyma.Labels = map[string]string{}
	}
	kb.kyma.Labels[key] = value
	return kb
}

// WithChannel sets v1beta2.Kyma.Spec.Channel
func (kb KymaBuilder) WithChannel(channel string) KymaBuilder {
	kb.kyma.Spec.Channel = channel
	return kb
}

// WithCondition adds a metav1.Condition to v1beta2.Kyma.Status.Conditions
func (kb KymaBuilder) WithCondition(condition metav1.Condition) KymaBuilder {
	if kb.kyma.Status.Conditions == nil {
		kb.kyma.Status.Conditions = []metav1.Condition{}
	}
	kb.kyma.Status.Conditions = append(kb.kyma.Status.Conditions, condition)
	return kb
}

// Build returns the built v1beta2.Kyma
func (kb KymaBuilder) Build() v1beta2.Kyma {
	return *kb.kyma
}
