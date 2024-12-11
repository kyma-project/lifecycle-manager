package gatewaysecret_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"
)

func TestParseLastModifiedTime(t *testing.T) {
	tests := []struct {
		name    string
		secret  *apicorev1.Secret
		want    time.Time
		wantErr bool
	}{
		{
			name: "valid lastModifiedAt annotation",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						"lastModifiedAt": "2024-11-01T00:00:00Z",
					},
				},
			},
			want:    time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name: "missing lastModifiedAt annotation",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			want:    time.Time{},
			wantErr: true,
		},
		{
			name: "invalid lastModifiedAt annotation key",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						"LastModifiedAt": "2024-11-01T00:00:00Z",
					},
				},
			},
			want:    time.Time{},
			wantErr: true,
		},
		{
			name: "invalid lastModifiedAt annotation time format",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						"lastModifiedAt": "2024-11-01T00:00:00",
					},
				},
			},
			want:    time.Time{},
			wantErr: true,
		},
	}
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			got, err := gatewaysecret.ParseLastModifiedFunc(testcase.secret)
			if (err != nil) != testcase.wantErr {
				t.Errorf("ParseLastModifiedTime() error = %v, wantErr %v", err, testcase.wantErr)
				return
			}
			if !reflect.DeepEqual(got, testcase.want) {
				t.Errorf("ParseLastModifiedTime() got = %v, want %v", got, testcase.want)
			}
		})
	}
}

func TestGetGatewaySecret_Error(t *testing.T) {
	clnt := fake.NewClientBuilder().Build()

	_, err := gatewaysecret.GetGatewaySecret(context.TODO(), clnt)

	require.Error(t, err)
}

func TestParseLastModifiedTime_MissingAnnotation(t *testing.T) {
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	_, err := gatewaysecret.ParseLastModifiedFunc(secret)

	require.Error(t, err)
}

func TestParseLastModifiedTime_InvalidTimeFormat(t *testing.T) {
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				"lastModifiedAt": "invalid-time-format",
			},
		},
	}

	_, err := gatewaysecret.ParseLastModifiedFunc(secret)

	require.Error(t, err)
}
