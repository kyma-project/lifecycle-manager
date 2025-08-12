package chartreader_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/chartreader"
)

type fakeClient struct {
	client.Client
}

func TestService_RunResourceOperationWithGroupedErrors(t *testing.T) {
	tests := []struct {
		name      string
		resources []client.Object
		op        chartreader.ResourceOperation
		wantErr   bool
	}{
		{
			name: "all succeed",
			resources: []client.Object{
				&unstructured.Unstructured{},
				&unstructured.Unstructured{},
			},
			op: func(ctx context.Context, clt client.Client, resource client.Object) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "one fails",
			resources: []client.Object{
				&unstructured.Unstructured{},
				&unstructured.Unstructured{},
			},
			op: func(ctx context.Context, clt client.Client, resource client.Object) error {
				return errors.New("fail")
			},
			wantErr: true,
		},
		{
			name:      "empty resources",
			resources: nil,
			op: func(ctx context.Context, clt client.Client, resource client.Object) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &chartreader.Service{}
			err := s.RunResourceOperationWithGroupedErrors(t.Context(), &fakeClient{}, tt.resources, tt.op)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failed to run resource operation")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_GetRawManifestUnstructuredResources(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)
	manifestPath := tmpDir + "/resources.yaml"
	content := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
`
	err := os.WriteFile(manifestPath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	tests := []struct {
		name    string
		dir     string
		want    []string
		wantErr bool
	}{
		{
			name:    "success",
			dir:     tmpDir,
			want:    []string{"ConfigMap", "Secret"},
			wantErr: false,
		},
		{
			name:    "dir not found",
			dir:     "/not/exist",
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid yaml",
			dir: func() string {
				d := t.TempDir()
				err := os.WriteFile(d+"/resources.yaml", []byte("not: [valid"), 0o600)
				if err != nil {
					return ""
				}
				return d
			}(),
			want:    nil,
			wantErr: true,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			s := chartreader.NewService(testCase.dir)
			got, err := s.GetRawManifestUnstructuredResources()
			if (err != nil) != testCase.wantErr {
				t.Errorf("GetRawManifestUnstructuredResources() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !testCase.wantErr {
				var kinds []string
				for _, r := range got {
					kinds = append(kinds, r.GetKind())
				}
				if !reflect.DeepEqual(kinds, testCase.want) {
					t.Errorf("GetRawManifestUnstructuredResources() got = %v, want %v", kinds, testCase.want)
				}
			}
		})
	}
}
