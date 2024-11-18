package watcher_test

import (
	"testing"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

func TestIsGatewaySecretNewerThanWatcherCert(t *testing.T) {
	type args struct {
		gatewaySecret *apicorev1.Secret
		watcherSecret *apicorev1.Secret
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Gateway secret is not newer than watcher cert",
			args: args{
				gatewaySecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{
							"lastModifiedAt": "2024-11-01T00:00:00Z",
						},
					},
				},
				watcherSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						CreationTimestamp: apimetav1.Time{
							Time: time.Date(2024, 11, 1, 0, 0, 5, 0, time.UTC),
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Gateway secret is newer than watcher cert",
			args: args{
				gatewaySecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{
							"lastModifiedAt": "2024-11-01T00:00:05Z",
						},
					},
				},
				watcherSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						CreationTimestamp: apimetav1.Time{
							Time: time.Date(2024, 11, 0o1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Invalid lastModifiedAt annotation in gateway secret",
			args: args{
				gatewaySecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				watcherSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						CreationTimestamp: apimetav1.Time{
							Time: time.Date(2024, 11, 0o1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Missing lastModifiedAt annotation in gateway secret",
			args: args{
				gatewaySecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{
							"lastModifiedAt": "invalid",
						},
					},
				},
				watcherSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						CreationTimestamp: apimetav1.Time{
							Time: time.Date(2024, 11, 0o1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := watcher.IsGatewaySecretNewerThanWatcherCert(tt.args.gatewaySecret, tt.args.watcherSecret); got != tt.want {
				t.Errorf("IsGatewaySecretNewerThanWatcherCert() = %v, want %v", got, tt.want)
			}
		})
	}
}
