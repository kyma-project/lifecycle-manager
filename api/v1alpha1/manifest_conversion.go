package v1alpha1

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

var ErrSingleInstallOnly = errors.New("v1beta1 only supports a single install at a time")

// ConvertTo converts this CronJob to the Hub version.
func (src *Manifest) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Manifest)

	dst.ObjectMeta = src.ObjectMeta

	if len(src.Spec.Installs) != 1 {
		return ErrSingleInstallOnly
	}

	for _, install := range src.Spec.Installs {
		install = InstallInfo{
			Source: install.Source,
			Name:   install.Name,
		}
	}

	dst.Spec.Config = v1beta1.ImageSpec{
		Repo:               src.Spec.Config.Repo,
		Name:               src.Spec.Config.Name,
		Ref:                src.Spec.Config.Ref,
		Type:               v1beta1.RefTypeMetadata(src.Spec.Config.Type),
		CredSecretSelector: src.Spec.Config.CredSecretSelector,
	}

	dst.Spec.Remote = src.Spec.Remote

	dst.Spec.Resource = src.Spec.Resource.DeepCopy()

	dst.Status = v1beta1.ManifestStatus(src.Status)

	return nil
}

// ConvertFrom converts from the Hub version to this version.
//
//nolint:revive
func (dst *Manifest) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Manifest)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Installs = []InstallInfo{{
		Source: src.Spec.Install.Source,
		Name:   src.Spec.Install.Name,
	}}

	dst.Spec.Remote = src.Spec.Remote

	dst.Spec.Config = ImageSpec{
		Repo:               src.Spec.Config.Repo,
		Name:               src.Spec.Config.Name,
		Ref:                src.Spec.Config.Ref,
		Type:               RefTypeMetadata(src.Spec.Config.Type),
		CredSecretSelector: src.Spec.Config.CredSecretSelector,
	}

	dst.Spec.Resource = src.Spec.Resource

	dst.Status = ManifestStatus(src.Status)

	return nil
}
