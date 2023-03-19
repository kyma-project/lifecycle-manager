package v2_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	mockV2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/record"
)

type stubRenderer struct {
	Data        []byte
	Err         error
	RenderCount int
}

func (t *stubRenderer) Initialize(_ Object) error                             { panic("not supported") }
func (t *stubRenderer) EnsurePrerequisites(_ context.Context, _ Object) error { panic("not supported") }
func (t *stubRenderer) RemovePrerequisites(_ context.Context, _ Object) error { panic("not supported") }

func (t *stubRenderer) Render(_ context.Context, _ Object) ([]byte, error) {
	t.RenderCount++
	if t.Err != nil {
		return nil, t.Err
	}
	return t.Data, nil
}

var errRandom = errors.New("random render error")

//nolint:funlen
func TestWrapWithRendererCache(t *testing.T) {
	t.Parallel()
	type args struct {
		spec *Spec
	}
	defaultArgs := args{
		spec: &Spec{
			ManifestName: "test-manifest",
			Path:         "test-path",
			Values:       map[string]any{"test-key": "test-value"},
			Mode:         RenderModeHelm,
		},
	}
	tests := []struct {
		name                   string
		args                   args
		callRenderTimes        int
		want                   []byte
		err                    error
		expectedEventsContains []string
		manifestCache          ManifestCache
	}{
		{
			"simple rendering that is cached",
			defaultArgs,
			2,
			[]byte("test-data"),
			nil,
			[]string{},
			ManifestCache(t.TempDir()),
		},
		{
			"simple rendering skip cache",
			defaultArgs,
			2,
			[]byte("test-data"),
			nil,
			[]string{},
			NoManifestCache,
		},
		{
			"render fails",
			defaultArgs,
			1,
			nil,
			errRandom,
			[]string{"RenderNonCached"},
			ManifestCache(t.TempDir()),
		},
	}

	for _, tt := range tests {
		testRun := tt
		t.Run(
			testRun.name, func(t *testing.T) {
				t.Parallel()
				assertions := assert.New(t)
				recorder := record.NewFakeRecorder(1)
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				renderer := &stubRenderer{Data: testRun.want, Err: testRun.err}

				mockObject := mockV2.NewMockObject(ctrl)
				status := Status{}
				mockObject.EXPECT().GetStatus().AnyTimes().Return(status)
				mockObject.EXPECT().SetStatus(gomock.AssignableToTypeOf(Status{})).AnyTimes().DoAndReturn(
					func(s Status) { status = s },
				)

				cachedRenderer := WrapWithRendererCache(
					renderer, testRun.args.spec,
					&Options{EventRecorder: recorder, ManifestCache: testRun.manifestCache},
				)

				var manifest []byte
				var err error
				for renderIter := 0; renderIter < testRun.callRenderTimes; renderIter++ {
					manifest, err = cachedRenderer.Render(context.Background(), mockObject)

					if err != nil {
						assertions.Contains(<-recorder.Events, testRun.expectedEventsContains[renderIter])
						assertions.Error(err, testRun.err, "error from render should match")
						assertions.Equal(StateError, status.State)
						assertions.NotEmpty(status.LastOperation.Operation)
						assertions.NotEmpty(status.LastOperation.LastUpdateTime)
						assertions.ErrorContains(err, status.LastOperation.Operation)
						return
					}

					desiredRenders := 1
					if testRun.manifestCache == NoManifestCache {
						desiredRenders = renderIter + 1
					}
					assertions.Equal(desiredRenders, renderer.RenderCount)
					assertions.Equal(manifest, testRun.want)
				}
			},
		)
	}
}
