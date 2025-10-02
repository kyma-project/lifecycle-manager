package sync_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/templatelookup"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
)

func TestNeedToUpdate(t *testing.T) {
	type args struct {
		manifestInCluster *v1beta2.Manifest
		newManifest       *v1beta2.Manifest
		moduleStatus      *v1beta2.ModuleStatus
		module            *modulecommon.Module
	}
	const trackedModuleTemplateGeneration = 1
	const updatedModuleTemplateGeneration = 2
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"When manifest in cluster is nil, expect need to update",
			args{
				nil,
				&v1beta2.Manifest{},
				&v1beta2.ModuleStatus{},
				&modulecommon.Module{},
			},
			true,
		},
		{
			"When manifest in cluster is nil and module is unmanaged, expect no update",
			args{
				nil,
				&v1beta2.Manifest{},
				&v1beta2.ModuleStatus{},
				&modulecommon.Module{
					IsUnmanaged: true,
				},
			},
			false,
		},
		{
			"When module status is nil, expect need to update",
			args{
				&v1beta2.Manifest{},
				&v1beta2.Manifest{},
				nil,
				&modulecommon.Module{
					IsUnmanaged: true,
				},
			},
			true,
		},
		{
			"When new module version available, expect need to update",
			args{
				&v1beta2.Manifest{},
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Labels: map[string]string{shared.ChannelLabel: "regular"},
					},
					Spec: v1beta2.ManifestSpec{Version: "0.2"},
				},
				&v1beta2.ModuleStatus{
					Version: "0.1",
					Channel: "regular",
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				},
				&modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: trackedModuleTemplateGeneration,
							},
						},
					},
				},
			},
			true,
		},
		{
			"When channel switch, expect need to update",
			args{
				&v1beta2.Manifest{},
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Labels: map[string]string{shared.ChannelLabel: "fast"},
					},
					Spec: v1beta2.ManifestSpec{Version: "0.1"},
				}, &v1beta2.ModuleStatus{
					Version: "0.1", Channel: "regular", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				},
				&modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: trackedModuleTemplateGeneration,
							},
						},
					},
				},
			},
			true,
		},
		{
			"When cluster Manifest in divergent state, expect need to update",
			args{
				&v1beta2.Manifest{
					Status: shared.Status{
						State: "Warning",
					},
				},
				&v1beta2.Manifest{},
				&v1beta2.ModuleStatus{
					State: "Ready", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				}, &modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: trackedModuleTemplateGeneration,
							},
						},
					},
				},
			},
			true,
		},
		{
			"When cluster Manifest and module both unmanaged, expect no update",
			args{
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{shared.UnmanagedAnnotation: "true"},
					},
					Status: shared.Status{
						State: "Ready",
					},
				},
				&v1beta2.Manifest{},
				&v1beta2.ModuleStatus{
					State: "Ready", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				}, &modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: trackedModuleTemplateGeneration,
							},
						},
					},
					IsUnmanaged: true,
				},
			},
			false,
		},
		{
			"When cluster Manifest is unmanaged and module is managed, expect no update",
			args{
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{shared.UnmanagedAnnotation: "true"},
					},
					Status: shared.Status{
						State: "Ready",
					},
				},
				&v1beta2.Manifest{},
				&v1beta2.ModuleStatus{
					State: "Ready", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				}, &modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: trackedModuleTemplateGeneration,
							},
						},
					},
					IsUnmanaged: false,
				},
			},
			false,
		},
		{
			"When cluster Manifest is managed and module is unmanaged, expect update",
			args{
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Labels: map[string]string{shared.ChannelLabel: "fast"},
					},
					Status: shared.Status{
						State: "Ready",
					},
				},
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Labels: map[string]string{shared.ChannelLabel: "fast"},
					},
				},
				&v1beta2.ModuleStatus{
					State: "Ready", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				}, &modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: trackedModuleTemplateGeneration,
							},
						},
					},
					IsUnmanaged: true,
				},
			},
			true,
		},
		{
			"When no update required, expect no update",
			args{
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Labels: map[string]string{shared.ChannelLabel: "regular"},
					},
					Spec: v1beta2.ManifestSpec{Version: "0.1"},
					Status: shared.Status{
						State: "Ready",
					},
				},
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Labels: map[string]string{shared.ChannelLabel: "regular"},
					},
					Spec: v1beta2.ManifestSpec{Version: "0.1"},
				},
				&v1beta2.ModuleStatus{
					State: "Ready", Version: "0.1", Channel: "regular", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				}, &modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: trackedModuleTemplateGeneration,
							},
						},
					},
					IsUnmanaged: false,
				},
			},
			false,
		},
		{
			"When moduleTemplate Generation updated, expect update",
			args{
				&v1beta2.Manifest{
					Status: shared.Status{
						State: "Ready",
					},
					Spec: v1beta2.ManifestSpec{Version: "0.1"},
				},
				&v1beta2.Manifest{
					ObjectMeta: apimetav1.ObjectMeta{
						Labels: map[string]string{shared.ChannelLabel: "regular"},
					},
					Spec: v1beta2.ManifestSpec{Version: "0.1"},
				},
				&v1beta2.ModuleStatus{
					State: "Ready", Version: "0.1", Channel: "regular", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				},
				&modulecommon.Module{
					TemplateInfo: &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: &v1beta2.ModuleTemplate{
							ObjectMeta: apimetav1.ObjectMeta{
								Generation: updatedModuleTemplateGeneration,
							},
						},
					},
					IsUnmanaged: false,
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, sync.NeedToUpdate(tt.args.manifestInCluster, tt.args.newManifest,
				tt.args.moduleStatus, tt.args.module), "needToUpdate(%v, %v, %v)",
				tt.args.manifestInCluster, tt.args.newManifest,
				tt.args.moduleStatus)
		})
	}
}
