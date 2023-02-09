package v1alpha1

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this CronJob to the Hub version.
func (m *Manifest) ConvertTo(hub conversion.Hub) error {
	dst := hub.(*v1beta1.Manifest)

	dst.ObjectMeta = m.ObjectMeta

	dst.Spec.Installs = make([]v1beta1.InstallInfo, 0, len(m.Spec.Installs))
	for _, install := range m.Spec.Installs {
		dst.Spec.Installs = append(
			dst.Spec.Installs, v1beta1.InstallInfo{
				Source: install.Source,
				Name:   install.Name,
			},
		)
	}

	dst.Spec.Config = v1beta1.ImageSpec{
		Repo:               m.Spec.Config.Repo,
		Name:               m.Spec.Config.Name,
		Ref:                m.Spec.Config.Ref,
		Type:               v1beta1.RefTypeMetadata(m.Spec.Config.Type),
		CredSecretSelector: m.Spec.Config.CredSecretSelector,
	}

	dst.Spec.Remote = m.Spec.Remote

	dst.Spec.Resource = m.Spec.Resource.DeepCopy()

	dst.Status = v1beta1.ManifestStatus(m.Status)

	return nil
}

// ConvertFrom converts from the Hub version to this version.
func (m *Manifest) ConvertFrom(hub conversion.Hub) error {
	src := hub.(*v1beta1.Manifest)

	m.ObjectMeta = src.ObjectMeta

	m.Spec.Installs = make([]InstallInfo, 0, len(src.Spec.Installs))
	for _, install := range src.Spec.Installs {
		m.Spec.Installs = append(
			m.Spec.Installs, InstallInfo{
				Source: install.Source,
				Name:   install.Name,
			},
		)
	}

	m.Spec.Remote = src.Spec.Remote

	m.Spec.Config = ImageSpec{
		Repo:               src.Spec.Config.Repo,
		Name:               src.Spec.Config.Name,
		Ref:                src.Spec.Config.Ref,
		Type:               RefTypeMetadata(src.Spec.Config.Type),
		CredSecretSelector: src.Spec.Config.CredSecretSelector,
	}

	m.Spec.Resource = src.Spec.Resource.DeepCopy()

	m.Status = ManifestStatus(src.Status)

	return nil
}
