package v1beta2_test

import (
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TestWatcher_GetManagerName(t *testing.T) {
	tests := []struct {
		name    string
		watcher *v1beta2.Watcher
		want    string
	}{
		{
			name: "should return spec.Manager when set",
			watcher: &v1beta2.Watcher{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ManagedBy: "label-value",
					},
				},
				Spec: v1beta2.WatcherSpec{
					Manager: "spec-value",
				},
			},
			want: "spec-value",
		},
		{
			name: "should fallback to label when spec.Manager is empty",
			watcher: &v1beta2.Watcher{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ManagedBy: "label-value",
					},
				},
				Spec: v1beta2.WatcherSpec{
					Manager: "",
				},
			},
			want: "label-value",
		},
		{
			name: "should return empty string when both are not set",
			watcher: &v1beta2.Watcher{
				ObjectMeta: apimetav1.ObjectMeta{},
				Spec: v1beta2.WatcherSpec{
					Manager: "",
				},
			},
			want: "",
		},
		{
			name: "should return empty string when labels are nil",
			watcher: &v1beta2.Watcher{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: nil,
				},
				Spec: v1beta2.WatcherSpec{
					Manager: "",
				},
			},
			want: "",
		},
		{
			name: "should prioritize spec.Manager over label",
			watcher: &v1beta2.Watcher{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ManagedBy: "lifecycle-manager",
					},
				},
				Spec: v1beta2.WatcherSpec{
					Manager: "custom-manager",
				},
			},
			want: "custom-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.watcher.GetManagerName()
			if got != tt.want {
				t.Errorf("GetManagerName() = %v, want %v", got, tt.want)
			}
		})
	}
}
