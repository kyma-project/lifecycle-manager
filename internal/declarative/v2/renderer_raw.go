package v2

import (
	"context"
	"os"

	"k8s.io/client-go/tools/record"
)

func NewRawRenderer(
	spec *Spec,
	options *Options,
) Renderer {
	return &RawRenderer{
		EventRecorder: options.EventRecorder,
		Path:          spec.Path,
	}
}

type RawRenderer struct {
	record.EventRecorder
	Path string
}

func (r *RawRenderer) Initialize(_ Object) error {
	return nil
}

func (r *RawRenderer) EnsurePrerequisites(_ context.Context, _ Object) error {
	return nil
}

func (r *RawRenderer) Render(_ context.Context, obj Object) ([]byte, error) {
	status := obj.GetStatus()
	manifest, err := os.ReadFile(r.Path)
	if err != nil {
		r.Event(obj, "Warning", "ReadRawManifest", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, err
	}
	return manifest, nil
}

func (r *RawRenderer) RemovePrerequisites(_ context.Context, _ Object) error {
	return nil
}
