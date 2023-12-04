package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	apiV1beta2 "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/api/v1beta2"
)

func (src *KymaInCtrlRuntime) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v1beta2.KymaInCtrlRuntime)
	if !ok {
		return v1beta2.ErrTypeAssertKyma
	}

	dst.ObjectMeta = src.ObjectMeta
	if dst.ObjectMeta.Labels == nil {
		dst.ObjectMeta.Labels = make(map[string]string)
	}
	if !src.Spec.Sync.Enabled {
		dst.ObjectMeta.Labels[shared.SyncLabel] = apiV1beta2.DisableLabelValue
	} else {
		dst.ObjectMeta.Labels[shared.SyncLabel] = apiV1beta2.EnableLabelValue
	}
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Modules = src.Spec.Modules
	dst.Status = src.Status

	return nil
}

//nolint:stylecheck
func (dst *KymaInCtrlRuntime) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1beta2.KymaInCtrlRuntime)
	if !ok {
		return v1beta2.ErrTypeAssertKyma
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Channel = src.Spec.Channel
	dst.Spec.Modules = src.Spec.Modules
	if src.HasSyncLabelEnabled() {
		dst.Spec.Sync.Enabled = true
	} else {
		dst.Spec.Sync.Enabled = false
	}
	dst.Status = src.Status

	return nil
}
