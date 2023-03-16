package v2_test

import (
	"context"
	. "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	mockV2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2/mock"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/record"
)

func TestNewRawRenderer(t *testing.T) {
	t.Parallel()
	recorder := record.NewFakeRecorder(1)
	type args struct {
		spec    *Spec
		options *Options
	}
	tests := []struct {
		name string
		args args
		want Renderer
	}{
		{
			"simple new test",
			args{
				spec: &Spec{
					Path: "test-Path",
				},
				options: &Options{
					EventRecorder: recorder,
				},
			},
			&RawRenderer{
				EventRecorder: recorder,
				Path:          "test-Path",
			},
		},
	}
	for _, tt := range tests {
		testRun := tt
		t.Run(
			testRun.name, func(t *testing.T) {
				t.Parallel()
				if got := NewRawRenderer(testRun.args.spec, testRun.args.options); !reflect.DeepEqual(
					got, testRun.want,
				) {
					t.Errorf("NewRawRenderer() = %v, want %v", got, testRun.want)
				}
			},
		)
	}
}

//nolint:funlen
func TestRawRenderer_EnsurePrerequisites(t *testing.T) {
	t.Parallel()
	recorder := record.NewFakeRecorder(1)
	type fields struct {
		recorder record.EventRecorder
		path     string
	}
	//nolint:containedctx
	type args struct {
		in0 context.Context
		in1 Object
	}
	tests := []struct {
		name                string
		fields              fields
		args                args
		wantErr             bool
		removePrerequisites bool
	}{
		{
			"noop",
			fields{
				recorder: recorder,
				path:     "test-Path",
			},
			args{
				in0: context.Background(),
				in1: nil,
			},
			false,
			false,
		},
		{
			"noop",
			fields{
				recorder: recorder,
				path:     "test-Path",
			},
			args{
				in0: context.Background(),
				in1: nil,
			},
			false,
			true,
		},
	}
	for _, tt := range tests {
		testRun := tt
		t.Run(
			testRun.name, func(t *testing.T) {
				t.Parallel()
				rawRendeer := &RawRenderer{
					EventRecorder: testRun.fields.recorder,
					Path:          testRun.fields.path,
				}
				if testRun.removePrerequisites {
					if err := rawRendeer.RemovePrerequisites(
						testRun.args.in0, testRun.args.in1,
					); (err != nil) != testRun.wantErr {
						t.Errorf("EnsurePrerequisites() error = %v, wantErr %v", err, testRun.wantErr)
					}
				} else {
					if err := rawRendeer.EnsurePrerequisites(
						testRun.args.in0, testRun.args.in1,
					); (err != nil) != testRun.wantErr {
						t.Errorf("EnsurePrerequisites() error = %v, wantErr %v", err, testRun.wantErr)
					}
				}
			},
		)
	}
}

func TestRawRenderer_Initialize(t *testing.T) {
	t.Parallel()
	recorder := record.NewFakeRecorder(1)
	type fields struct {
		recorder record.EventRecorder
		path     string
	}
	type args struct {
		in0 Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"noop",
			fields{
				recorder: recorder,
				path:     "test-Path",
			},
			args{in0: nil},
			false,
		},
	}
	for _, tt := range tests {
		testRun := tt
		t.Run(
			testRun.name, func(t *testing.T) {
				t.Parallel()
				r := &RawRenderer{
					EventRecorder: testRun.fields.recorder,
					Path:          testRun.fields.path,
				}
				if err := r.Initialize(testRun.args.in0); (err != nil) != testRun.wantErr {
					t.Errorf("Initialize() error = %v, wantErr %v", err, testRun.wantErr)
				}
			},
		)
	}
}

//nolint:funlen
func TestRawRenderer_Render(t *testing.T) {
	t.Parallel()
	type fields struct {
		path string
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			"fail on file read",
			fields{path: "test-Path-does-not-exist"},
			nil,
			true,
		},
		{
			"succeed",
			fields{path: filepath.Join(t.TempDir(), "test-raw-render")},
			[]byte("test: true"),
			false,
		},
	}
	for _, tt := range tests {
		testRun := tt
		t.Run(
			testRun.name, func(t *testing.T) {
				assertions := assert.New(t)
				t.Parallel()
				recorder := record.NewFakeRecorder(1)
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockObject := mockV2.NewMockObject(ctrl)
				status := Status{}
				mockObject.EXPECT().GetStatus().AnyTimes().Return(status)
				mockObject.EXPECT().SetStatus(gomock.AssignableToTypeOf(Status{})).AnyTimes().DoAndReturn(
					func(s Status) {
						status = s
					},
				)

				renderer := &RawRenderer{
					EventRecorder: recorder,
					Path:          testRun.fields.path,
				}
				if !testRun.wantErr {
					_, err := os.Create(testRun.fields.path)
					assertions.NoError(err)
					assertions.NoError(os.WriteFile(testRun.fields.path, testRun.want, os.ModeTemporary))
					defer func() {
						assertions.NoError(os.Remove(testRun.fields.path))
					}()
				}
				got, err := renderer.Render(context.Background(), mockObject)
				if (err != nil) != testRun.wantErr {
					t.Errorf("Render() error = %v, wantErr %v", err, testRun.wantErr)
					return
				}
				if testRun.wantErr {
					if !assertions.Contains(<-recorder.Events, "ReadRawManifest") {
						t.Error("error did not cause event")
					}
					if status.State != StateError {
						t.Errorf("status of object was not %s, but %s", StateError, status.State)
					}
					if status.LastOperation.Operation != err.Error() {
						t.Errorf(
							"Last Operation should contain the error %v, but was %v", err,
							status.LastOperation.Operation,
						)
					}
				}
				if !reflect.DeepEqual(got, testRun.want) {
					t.Errorf("Render() got = %v, want %v", got, testRun.want)
				}
			},
		)
	}
}
