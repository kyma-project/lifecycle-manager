package builder

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type ManifestBuilder struct {
	manifest *v1beta2.Manifest
}

// NewManifestBuilder returns a ManifestBuilder with v1beta2.Manifest initialized defaults.
func NewManifestBuilder() ManifestBuilder {
	return ManifestBuilder{
		manifest: &v1beta2.Manifest{
			TypeMeta: apimetav1.TypeMeta{
				APIVersion: v1beta2.GroupVersion.String(),
				Kind:       string(shared.ManifestKind),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      random.Name(),
				Namespace: apimetav1.NamespaceDefault,
			},
			Spec:   v1beta2.ManifestSpec{},
			Status: shared.Status{},
		},
	}
}

// WithName sets v1beta2.Manifest.ObjectMeta.Name.
func (mb ManifestBuilder) WithName(name string) ManifestBuilder {
	mb.manifest.Name = name
	return mb
}

// WithNamespace sets v1beta2.Manifest.ObjectMeta.Namespace.
func (mb ManifestBuilder) WithNamespace(namespace string) ManifestBuilder {
	mb.manifest.Namespace = namespace
	return mb
}

// WithAnnotation adds an annotation to v1beta2.Manifest.ObjectMeta.Annotation.
func (mb ManifestBuilder) WithAnnotation(key string, value string) ManifestBuilder {
	if mb.manifest.Annotations == nil {
		mb.manifest.Annotations = map[string]string{}
	}
	mb.manifest.Annotations[key] = value
	return mb
}

// WithGeneration sets v1beta2.Manifest.ObjectMeta.Generation.
func (mb ManifestBuilder) WithGeneration(generation int) ManifestBuilder {
	mb.manifest.Generation = int64(generation)
	return mb
}

// WithLabel adds a label to v1beta2.Manifest.ObjectMeta.Labels.
func (mb ManifestBuilder) WithLabel(key string, value string) ManifestBuilder {
	if mb.manifest.Labels == nil {
		mb.manifest.Labels = map[string]string{}
	}
	mb.manifest.Labels[key] = value
	return mb
}

// WithSpec sets v1beta2.Manifest.Spec.
func (mb ManifestBuilder) WithSpec(spec v1beta2.ManifestSpec) ManifestBuilder {
	mb.manifest.Spec = spec
	return mb
}

func (mb ManifestBuilder) WithChannel(channel string) ManifestBuilder {
	return mb.WithLabel(shared.ChannelLabel, channel)
}

func (mb ManifestBuilder) IsMandatoryModule() ManifestBuilder {
	return mb.WithLabel(shared.IsMandatoryModule, "true")
}

func (mb ManifestBuilder) WithVersion(version string) ManifestBuilder {
	mb.manifest.Spec.Version = version
	return mb
}

// WithStatus sets v1beta2.Manifest.Status.
func (mb ManifestBuilder) WithStatus(status shared.Status) ManifestBuilder {
	mb.manifest.Status = status
	return mb
}

// WithFinalizers sets v1beta2.Manifest.Finalizers.
func (mb ManifestBuilder) WithFinalizers(finalizers []string) ManifestBuilder {
	mb.manifest.Finalizers = finalizers
	return mb
}

func (mb ManifestBuilder) WithResource(resource *unstructured.Unstructured) ManifestBuilder {
	mb.manifest.Spec.Resource = resource
	return mb
}

// Build returns the built v1beta2.Manifest.
func (mb ManifestBuilder) Build() *v1beta2.Manifest {
	return mb.manifest
}
