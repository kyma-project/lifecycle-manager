package v1beta1

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this to the Hub version.
func (src *Manifest) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.Manifest)

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}

// ConvertFrom converts from the Hub version to this version.
//
//nolint:revive,stylecheck
func (dst *Manifest) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.Manifest)

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}
