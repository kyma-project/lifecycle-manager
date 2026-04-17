package events_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta/events"
)

func Test_AffectedKymasOnDelete(t *testing.T) {
	tests := []struct {
		name  string
		mrm   *v1beta2.ModuleReleaseMeta
		kymas *v1beta2.KymaList
		want  []*types.NamespacedName
	}{
		{
			name: "no kymas present",
			mrm: &v1beta2.ModuleReleaseMeta{
				Spec: v1beta2.ModuleReleaseMetaSpec{
					ModuleName: "module",
					Channels: []v1beta2.ChannelVersionAssignment{
						{Channel: "regular", Version: "1.0.0"},
					},
				},
			},
			kymas: &v1beta2.KymaList{},
			want:  []*types.NamespacedName{},
		},
		{
			name: "kyma with affected module is requeued",
			mrm: &v1beta2.ModuleReleaseMeta{
				Spec: v1beta2.ModuleReleaseMetaSpec{
					ModuleName: "module",
					Channels: []v1beta2.ChannelVersionAssignment{
						{Channel: "regular", Version: "1.0.0"},
					},
				},
			},
			kymas: &v1beta2.KymaList{
				Items: []v1beta2.Kyma{
					{
						ObjectMeta: apimetav1.ObjectMeta{Name: "kyma-1", Namespace: "kcp-system"},
						Status: v1beta2.KymaStatus{
							Modules: []v1beta2.ModuleStatus{
								{
									Name:    "module",
									Channel: "regular",
								},
							},
						},
					},
					{
						ObjectMeta: apimetav1.ObjectMeta{Name: "kyma-2", Namespace: "kcp-system"},
						Status: v1beta2.KymaStatus{
							Modules: []v1beta2.ModuleStatus{
								{
									Name:    "other",
									Channel: "regular",
								},
							},
						},
					},
				},
			},
			want: []*types.NamespacedName{{Name: "kyma-1", Namespace: "kcp-system"}},
		},
		{
			name: "all kymas with module across all channels requeued on delete",
			mrm: &v1beta2.ModuleReleaseMeta{
				Spec: v1beta2.ModuleReleaseMetaSpec{
					ModuleName: "module",
					Channels: []v1beta2.ChannelVersionAssignment{
						{Channel: "regular", Version: "1.0.0"},
						{Channel: "fast", Version: "2.0.0"},
					},
				},
			},
			kymas: &v1beta2.KymaList{
				Items: []v1beta2.Kyma{
					{
						ObjectMeta: apimetav1.ObjectMeta{Name: "kyma-1", Namespace: "kcp-system"},
						Status: v1beta2.KymaStatus{
							Modules: []v1beta2.ModuleStatus{
								{
									Name:    "module",
									Channel: "regular",
								},
							},
						},
					},
					{
						ObjectMeta: apimetav1.ObjectMeta{Name: "kyma-2", Namespace: "kcp-system"},
						Status: v1beta2.KymaStatus{
							Modules: []v1beta2.ModuleStatus{
								{
									Name:    "module",
									Channel: "fast",
								},
							},
						},
					},
				},
			},
			want: []*types.NamespacedName{
				{Name: "kyma-1", Namespace: "kcp-system"},
				{Name: "kyma-2", Namespace: "kcp-system"},
			},
		},
		{
			name: "kyma channel falls back to kyma spec channel when module channel is empty",
			mrm: &v1beta2.ModuleReleaseMeta{
				Spec: v1beta2.ModuleReleaseMetaSpec{
					ModuleName: "module",
					Channels: []v1beta2.ChannelVersionAssignment{
						{Channel: "regular", Version: "1.0.0"},
					},
				},
			},
			kymas: &v1beta2.KymaList{
				Items: []v1beta2.Kyma{
					{
						ObjectMeta: apimetav1.ObjectMeta{Name: "kyma-1", Namespace: "kcp-system"},
						Spec:       v1beta2.KymaSpec{Channel: "regular"},
						Status:     v1beta2.KymaStatus{Modules: []v1beta2.ModuleStatus{{Name: "module", Channel: ""}}},
					},
				},
			},
			want: []*types.NamespacedName{{Name: "kyma-1", Namespace: "kcp-system"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := events.AffectedKymasOnDelete(tt.mrm, tt.kymas)
			require.Equal(t, tt.want, got)
		})
	}
}
