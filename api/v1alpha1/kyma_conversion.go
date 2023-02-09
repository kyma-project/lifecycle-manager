package v1alpha1

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *Kyma) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Kyma)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status
	return nil
}

func (dst *Kyma) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Kyma)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	dst.Status = src.Status
	return nil
}
