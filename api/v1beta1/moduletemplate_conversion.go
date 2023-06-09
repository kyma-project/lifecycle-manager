package v1beta1

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *ModuleTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.ModuleTemplate)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Data = src.Spec.Data
	dst.Spec.Descriptor = src.Spec.Descriptor
	dst.Spec.CustomStateCheck = src.Spec.CustomStateCheck
	return nil
}

//nolint:revive,stylecheck
func (dst *ModuleTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.ModuleTemplate)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Data = src.Spec.Data
	dst.Spec.Descriptor = src.Spec.Descriptor
	dst.Spec.CustomStateCheck = src.Spec.CustomStateCheck
	dst.Spec.Target = TargetRemote
	return nil
}
