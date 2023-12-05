package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	pkgapiv1beta2 "github.com/kyma-project/lifecycle-manager/pkg/api/v1beta2"
)

// ConvertTo converts this to the Hub version.
func (src *ManifestInCtrlRuntime) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*pkgapiv1beta2.ManifestInCtrlRuntime)
	if !ok {
		return pkgapiv1beta2.ErrTypeAssertManifest
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}

// ConvertFrom converts from the Hub version to this version.
//
//nolint:stylecheck
func (dst *ManifestInCtrlRuntime) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*pkgapiv1beta2.ManifestInCtrlRuntime)
	if !ok {
		return pkgapiv1beta2.ErrTypeAssertManifest
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}
