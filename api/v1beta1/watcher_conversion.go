package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func (src *Watcher) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.Watcher)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status
	return nil
}

//nolint:revive,stylecheck
func (dst *Watcher) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.Watcher)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status
	return nil
}
