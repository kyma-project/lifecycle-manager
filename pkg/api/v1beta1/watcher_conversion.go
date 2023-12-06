package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	pkgapiv1beta2 "github.com/kyma-project/lifecycle-manager/pkg/api/v1beta2"
)

func (src *WatcherInCtrlRuntime) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*pkgapiv1beta2.WatcherInCtrlRuntime)
	if !ok {
		return pkgapiv1beta2.ErrTypeAssertWatcher
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}

//nolint:stylecheck
func (dst *WatcherInCtrlRuntime) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*pkgapiv1beta2.WatcherInCtrlRuntime)
	if !ok {
		return pkgapiv1beta2.ErrTypeAssertWatcher
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}
