package watch

import (
	"reflect"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Test_diffModuleReleaseMetaChannels(t *testing.T) {
	type args struct {
		oldModuleReleaseMeta *v1beta2.ModuleReleaseMeta
		newModuleReleaseMeta *v1beta2.ModuleReleaseMeta
	}
	tests := []struct {
		name string
		args args
		want map[string]v1beta2.ChannelVersionAssignment
	}{
		{
			name: "Empty input",
			args: args{
				oldModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{},
						},
					},
				},
				newModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{},
						},
					},
				},
			},
			want: map[string]v1beta2.ChannelVersionAssignment{},
		},
		{
			name: "No difference same channels",
			args: args{
				oldModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.0.0",
							},
						},
					},
				},
				newModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.0.0",
							},
						},
					},
				},
			},
			want: map[string]v1beta2.ChannelVersionAssignment{},
		},
		{
			name: "One channel updated",
			args: args{
				oldModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.0.0",
							},
							{
								Channel: "fast",
								Version: "2.0.0",
							},
						},
					},
				},
				newModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.1.0",
							},
							{
								Channel: "fast",
								Version: "2.0.0",
							},
						},
					},
				},
			},
			want: map[string]v1beta2.ChannelVersionAssignment{
				"regular": {
					Channel: "regular",
					Version: "1.1.0",
				},
			},
		},
		{
			name: "All channels updated",
			args: args{
				oldModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.0.0",
							},
							{
								Channel: "fast",
								Version: "2.0.0",
							},
						},
					},
				},
				newModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.1.0",
							},
							{
								Channel: "fast",
								Version: "2.1.0",
							},
						},
					},
				},
			},
			want: map[string]v1beta2.ChannelVersionAssignment{
				"regular": {
					Channel: "regular",
					Version: "1.1.0",
				},
				"fast": {
					Channel: "fast",
					Version: "2.1.0",
				},
			},
		},
		{
			name: "New channel added",
			args: args{
				oldModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.0.0",
							},
						},
					},
				},
				newModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.0.0",
							},
							{
								Channel: "fast",
								Version: "2.0.0",
							},
						},
					},
				},
			},
			want: map[string]v1beta2.ChannelVersionAssignment{
				"fast": {
					Channel: "fast",
					Version: "2.0.0",
				},
			},
		},
		{
			name: "Channel removed",
			args: args{
				oldModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{
							{
								Channel: "regular",
								Version: "1.0.0",
							},
						},
					},
				},
				newModuleReleaseMeta: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{
						Channels: []v1beta2.ChannelVersionAssignment{},
					},
				},
			},
			want: map[string]v1beta2.ChannelVersionAssignment{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := diffModuleReleaseMetaChannels(tt.args.oldModuleReleaseMeta,
				tt.args.newModuleReleaseMeta); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("diffModuleReleaseMetaChannels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getAffectedKymas(t *testing.T) {
	type args struct {
		kymas                 *v1beta2.KymaList
		moduleName            string
		newChannelAssignments map[string]v1beta2.ChannelVersionAssignment
	}
	tests := []struct {
		name string
		args args
		want []*types.NamespacedName
	}{
		{
			name: "No Kymas present",
			args: args{
				kymas:                 &v1beta2.KymaList{},
				newChannelAssignments: map[string]v1beta2.ChannelVersionAssignment{},
			},
			want: []*types.NamespacedName{},
		},
		{
			name: "One Kyma with global channel affected",
			args: args{
				kymas: &v1beta2.KymaList{
					Items: []v1beta2.Kyma{
						{
							ObjectMeta: apimetav1.ObjectMeta{
								Name:      "test-kyma-1",
								Namespace: "kcp-system",
							},
							Spec: v1beta2.KymaSpec{
								Channel: "regular",
								Modules: []v1beta2.Module{
									{
										Name: "module",
									},
								},
							},
						},
						{
							ObjectMeta: apimetav1.ObjectMeta{
								Name:      "test-kyma-2",
								Namespace: "kcp-system",
							},
							Spec: v1beta2.KymaSpec{
								Channel: "regular",
								Modules: []v1beta2.Module{
									{
										Name:    "module",
										Channel: "fast",
									},
								},
							},
						},
					},
				},
				moduleName: "module",
				newChannelAssignments: map[string]v1beta2.ChannelVersionAssignment{
					"regular": {
						Channel: "regular",
						Version: "1.0.0",
					},
				},
			},
			want: []*types.NamespacedName{
				{
					Name:      "test-kyma-1",
					Namespace: "kcp-system",
				},
			},
		},
		{
			name: "One Kyma with module specific channel affected",
			args: args{
				kymas: &v1beta2.KymaList{
					Items: []v1beta2.Kyma{
						{
							ObjectMeta: apimetav1.ObjectMeta{
								Name:      "test-kyma-1",
								Namespace: "kcp-system",
							},
							Spec: v1beta2.KymaSpec{
								Channel: "fast",
								Modules: []v1beta2.Module{
									{
										Name:    "module",
										Channel: "regular",
									},
								},
							},
						},
						{
							ObjectMeta: apimetav1.ObjectMeta{
								Name:      "test-kyma-2",
								Namespace: "kcp-system",
							},
							Spec: v1beta2.KymaSpec{
								Channel: "regular",
								Modules: []v1beta2.Module{
									{
										Name:    "module",
										Channel: "fast",
									},
								},
							},
						},
					},
				},
				moduleName: "module",
				newChannelAssignments: map[string]v1beta2.ChannelVersionAssignment{
					"regular": {
						Channel: "regular",
						Version: "1.0.0",
					},
				},
			},
			want: []*types.NamespacedName{
				{
					Name:      "test-kyma-1",
					Namespace: "kcp-system",
				},
			},
		},
		{
			name: "No Kyma affected",
			args: args{
				kymas: &v1beta2.KymaList{
					Items: []v1beta2.Kyma{
						{
							ObjectMeta: apimetav1.ObjectMeta{
								Name:      "test-kyma-1",
								Namespace: "kcp-system",
							},
							Spec: v1beta2.KymaSpec{
								Channel: "regular",
								Modules: []v1beta2.Module{
									{
										Name: "module",
									},
								},
							},
						},
						{
							ObjectMeta: apimetav1.ObjectMeta{
								Name:      "test-kyma-2",
								Namespace: "kcp-system",
							},
							Spec: v1beta2.KymaSpec{
								Channel: "regular",
								Modules: []v1beta2.Module{
									{
										Name:    "module",
										Channel: "fast",
									},
								},
							},
						},
					},
				},
				moduleName: "module",
				newChannelAssignments: map[string]v1beta2.ChannelVersionAssignment{
					"experimental": {
						Channel: "experimental",
						Version: "1.0.0",
					},
				},
			},
			want: []*types.NamespacedName{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAffectedKymas(tt.args.kymas, tt.args.moduleName,
				tt.args.newChannelAssignments); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAffectedKymas() = %v, want %v", got, tt.want)
			}
		})
	}
}
