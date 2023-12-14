package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func (src *Watcher) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v1beta2.Watcher)
	if !ok {
		return v1beta2.ErrTypeAssertWatcher
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}

//nolint:stylecheck // stick to controller-runtime.conversion naming convention
func (dst *Watcher) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1beta2.Watcher)
	if !ok {
		return v1beta2.ErrTypeAssertWatcher
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status

	return nil
}
