package v2

import (
	"context"
	"encoding/json"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func NewKustomizeRenderer(
	spec *Spec,
	options *Options,
) Renderer {
	var krustyOpts *krusty.Options

	if optionsFromValues, areKrustyOpts := spec.Values.(*krusty.Options); areKrustyOpts {
		krustyOpts = optionsFromValues
	} else {
		krustyOpts = krusty.MakeDefaultOptions()
		jsonValues, err := json.Marshal(spec.Values)
		if err == nil {
			if err := json.Unmarshal(jsonValues, krustyOpts); err != nil {
				krustyOpts = krusty.MakeDefaultOptions()
			}
		}
	}

	return &Kustomize{
		recorder: options.EventRecorder,
		path:     spec.Path,
		opts:     krustyOpts,
	}
}

type Kustomize struct {
	recorder record.EventRecorder
	path     string
	opts     *krusty.Options

	kustomizer *krusty.Kustomizer
	fs         filesys.FileSystem
}

func (k *Kustomize) Initialize(_ Object) error {
	k.kustomizer = krusty.MakeKustomizer(k.opts)

	// file system on which kustomize works on
	k.fs = filesys.MakeFsOnDisk()
	return nil
}

func (k *Kustomize) EnsurePrerequisites(_ context.Context, _ Object) error {
	return nil
}

func (k *Kustomize) Render(_ context.Context, obj Object) ([]byte, error) {
	status := obj.GetStatus()

	resMap, err := k.kustomizer.Run(k.fs, k.path)
	if err != nil {
		k.recorder.Event(obj, "Warning", "KustomizeRenderRun", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, err
	}

	manifest, err := resMap.AsYaml()
	if err != nil {
		k.recorder.Event(obj, "Warning", "KustomizeYAMLConversion", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
	}

	return manifest, err
}

func (k *Kustomize) RemovePrerequisites(_ context.Context, _ Object) error {
	return nil
}
