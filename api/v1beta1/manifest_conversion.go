package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// ConvertTo converts this to the Hub version.
func (src *Manifest) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v1beta2.Manifest)
	if !ok {
		return v1beta2.ErrTypeAssertManifest
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}

// ConvertFrom converts from the Hub version to this version.
//
//nolint:stylecheck // stick to controller-runtime.conversion naming convention
func (dst *Manifest) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1beta2.Manifest)
	if !ok {
		return v1beta2.ErrTypeAssertManifest
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}
