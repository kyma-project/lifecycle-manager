package v2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kyma-project/lifecycle-manager/pkg/types"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBufferSize                                          = 2048
	ConditionTypeRawManifestCRDs               ConditionType   = "RawManifestCRDs"
	ConditionReasonRawManifestCRDsAreAvailable ConditionReason = "RawManifestCRDsAvailable"
)

func NewRawRenderer(
	spec *Spec,
	clt Client,
	options *Options,
) Renderer {
	return &RawRenderer{
		EventRecorder: options.EventRecorder,
		Client:        clt,
		Path:          spec.Path,
	}
}

type RawRenderer struct {
	record.EventRecorder
	Client
	Path string
	crds []*unstructured.Unstructured
}

func (r *RawRenderer) prerequisiteCondition(object metav1.Object) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionTypeRawManifestCRDs),
		Reason:             string(ConditionReasonRawManifestCRDsAreAvailable),
		Status:             metav1.ConditionFalse,
		Message:            "CustomResourceDefinitions from raw manifest resources are installed and ready for use",
		ObservedGeneration: object.GetGeneration(),
	}
}

func (r *RawRenderer) Initialize(obj Object) error {
	status := obj.GetStatus()

	prerequisiteExists := meta.FindStatusCondition(status.Conditions, string(ConditionTypeRawManifestCRDs)) != nil
	if !prerequisiteExists {
		meta.SetStatusCondition(&status.Conditions, r.prerequisiteCondition(obj))
		obj.SetStatus(status)
		return ErrConditionsNotYetRegistered
	}

	return nil
}

func (r *RawRenderer) EnsurePrerequisites(ctx context.Context, obj Object) error {
	status := obj.GetStatus()
	if meta.IsStatusConditionTrue(
		status.Conditions, string(ConditionTypeRawManifestCRDs),
	) {
		return nil
	}
	manifestFile, err := os.Open(r.Path)
	if err != nil {
		r.Event(obj, "Warning", "ReadRawManifest", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}

	if err := r.getCRDs(manifestFile); err != nil {
		r.Event(obj, "Warning", "ParseRawManifest", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}
	if err := r.installCRDs(ctx); err != nil {
		r.Event(obj, "Warning", "InstallRawManifestCRDs", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}
	restMapper, _ := r.ToRESTMapper()
	meta.MaybeResetRESTMapper(restMapper)
	cond := r.prerequisiteCondition(obj)
	cond.Status = metav1.ConditionTrue
	r.Event(obj, "Normal", cond.Reason, cond.Message)
	meta.SetStatusCondition(&status.Conditions, cond)
	obj.SetStatus(status.WithOperation("CRDs are ready"))
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

func (r *RawRenderer) RemovePrerequisites(ctx context.Context, obj Object) error {
	crdLength := len(r.crds)
	if crdLength == 0 {
		return nil
	}
	status := obj.GetStatus()
	if err := r.uninstallCRDs(ctx); err != nil {
		r.Event(obj, "Warning", "CRDsUninstallation", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}
	return nil
}

func (r *RawRenderer) getCRDs(rawManifestReader io.Reader) error {
	r.crds = make([]*unstructured.Unstructured, 0)
	decoder := yaml.NewYAMLOrJSONDecoder(rawManifestReader, defaultBufferSize)
	for {
		resource := &unstructured.Unstructured{}
		err := decoder.Decode(resource)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if errors.Is(err, io.EOF) {
			break
		}
		crdGVK := apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition")
		if resource.GroupVersionKind() == crdGVK {
			r.crds = append(r.crds, resource)
			continue
		}
	}
	return nil
}

func (r *RawRenderer) installCRDs(ctx context.Context) error {
	crdCount := len(r.crds)
	errChan := make(chan error, crdCount)
	for idx := range r.crds {
		crd := r.crds[idx]
		go func() {
			errChan <- r.Patch(ctx, crd, client.Apply, client.FieldOwner(FieldOwnerDefault))
		}()
	}
	errs := make([]error, 0)
	for i := 0; i < crdCount; i++ {
		if err := <-errChan; err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("failed to install raw manifest CRDs: %w", types.NewMultiError(errs))
	}

	return nil
}

func (r *RawRenderer) uninstallCRDs(ctx context.Context) error {
	crdCount := len(r.crds)
	errChan := make(chan error, crdCount)
	for idx := range r.crds {
		crd := r.crds[idx]
		go func() {
			errChan <- r.Delete(ctx, crd, &client.DeleteOptions{})
		}()
	}
	errs := make([]error, 0)
	for i := 0; i < crdCount; i++ {
		if err := <-errChan; err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("failed to remove raw manifest CRDs: %w", types.NewMultiError(errs))
	}
	return nil
}
