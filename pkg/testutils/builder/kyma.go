package builder

import (
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type KymaBuilder struct {
	kyma *v1beta2.Kyma
}

// NewKymaBuilder returns a KymaBuilder with v1beta2.Kyma initialized defaults.
func NewKymaBuilder() KymaBuilder {
	return KymaBuilder{
		kyma: &v1beta2.Kyma{
			TypeMeta: apimetav1.TypeMeta{
				APIVersion: v1beta2.GroupVersion.String(),
				Kind:       string(shared.KymaKind),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      random.Name(),
				Namespace: apimetav1.NamespaceDefault,
			},
			Spec:   v1beta2.KymaSpec{},
			Status: v1beta2.KymaStatus{},
		},
	}
}

// WithName sets v1beta2.Kyma.ObjectMeta.Name.
func (kb KymaBuilder) WithName(name string) KymaBuilder {
	kb.kyma.ObjectMeta.Name = name
	return kb
}

// WithEnabledModule append module to v1beta2.Kyma.Spec.Modules.
func (kb KymaBuilder) WithEnabledModule(module v1beta2.Module) KymaBuilder {
	if kb.kyma.Spec.Modules == nil {
		kb.kyma.Spec.Modules = []v1beta2.Module{}
	}
	kb.kyma.Spec.Modules = append(kb.kyma.Spec.Modules, module)
	return kb
}

// WithNamePrefix sets v1beta2.Kyma.ObjectMeta.Name.
func (kb KymaBuilder) WithNamePrefix(prefix string) KymaBuilder {
	kb.kyma.ObjectMeta.Name = fmt.Sprintf("%s-%s", prefix, random.Name())
	return kb
}

// WithNamespace sets v1beta2.Kyma.ObjectMeta.Namespace.
func (kb KymaBuilder) WithNamespace(namespace string) KymaBuilder {
	kb.kyma.ObjectMeta.Namespace = namespace
	return kb
}

// WithAnnotation adds an annotation to v1beta2.Kyma.ObjectMeta.Annotation.
func (kb KymaBuilder) WithAnnotation(key string, value string) KymaBuilder {
	if kb.kyma.Annotations == nil {
		kb.kyma.Annotations = map[string]string{}
	}
	kb.kyma.Annotations[key] = value
	return kb
}

// WithLabel adds a label to v1beta2.Kyma.ObjectMeta.Labels.
func (kb KymaBuilder) WithLabel(key string, value string) KymaBuilder {
	if kb.kyma.Labels == nil {
		kb.kyma.Labels = map[string]string{}
	}
	kb.kyma.Labels[key] = value
	return kb
}

// WithChannel sets v1beta2.Kyma.Spec.Channel.
func (kb KymaBuilder) WithChannel(channel string) KymaBuilder {
	kb.kyma.Spec.Channel = channel
	return kb
}

// WithCondition adds a Condition to v1beta2.Kyma.Status.Conditions.
func (kb KymaBuilder) WithCondition(condition apimetav1.Condition) KymaBuilder {
	if kb.kyma.Status.Conditions == nil {
		kb.kyma.Status.Conditions = []apimetav1.Condition{}
	}
	kb.kyma.Status.Conditions = append(kb.kyma.Status.Conditions, condition)
	return kb
}

// WithCondition adds a ModuleStatus to v1beta2.Kyma.Status.Modules.
func (kb KymaBuilder) WithModuleStatus(moduleStatus v1beta2.ModuleStatus) KymaBuilder {
	if kb.kyma.Status.Modules == nil {
		kb.kyma.Status.Modules = []v1beta2.ModuleStatus{}
	}
	kb.kyma.Status.Modules = append(kb.kyma.Status.Modules, moduleStatus)
	return kb
}

// Build returns the built v1beta2.Kyma.
func (kb KymaBuilder) Build() *v1beta2.Kyma {
	return kb.kyma
}
