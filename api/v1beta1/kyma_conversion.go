package v1beta1

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *Kyma) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.Kyma)
	dst.ObjectMeta = src.ObjectMeta
	if !src.Spec.Sync.Enabled {
		if dst.ObjectMeta.Labels == nil {
			dst.ObjectMeta.Labels = make(map[string]string)
		}
		dst.ObjectMeta.Labels[v1beta2.SyncLabel] = v1beta2.DisableLabelValue
	}
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Modules = src.Spec.Modules
	dst.Status = src.Status
	return nil
}

//nolint:revive,stylecheck
func (dst *Kyma) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.Kyma)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Modules = src.Spec.Modules
	dst.Spec.Sync = Sync{}
	dst.Status = src.Status
	return nil
}
