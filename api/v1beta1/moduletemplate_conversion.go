package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func (src *ModuleTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v1beta2.ModuleTemplate)
	if !ok {
		return v1beta2.ErrTypeAssertModuleTemplate
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Data = src.Spec.Data
	dst.Spec.Descriptor = src.Spec.Descriptor
	dst.Spec.CustomStateCheck = src.Spec.CustomStateCheck
	return nil
}

//nolint:stylecheck // stick to controller-runtime.conversion naming convention
func (dst *ModuleTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1beta2.ModuleTemplate)
	if !ok {
		return v1beta2.ErrTypeAssertModuleTemplate
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Data = src.Spec.Data
	dst.Spec.Descriptor = src.Spec.Descriptor
	dst.Spec.CustomStateCheck = src.Spec.CustomStateCheck
	dst.Spec.Target = TargetRemote

	return nil
}
