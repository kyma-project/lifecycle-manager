package types

import (
	"errors"
	"fmt"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"ocm.software/ocm/api/ocm/compdesc"
)

var ErrDecode = errors.New("failed to decode component descriptor")

type Descriptor struct {
	*compdesc.ComponentDescriptor
}

func (d *Descriptor) SetGroupVersionKind(kind schema.GroupVersionKind) {
	d.Version = kind.Version
}

func (d *Descriptor) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "ocm.kyma-project.io",
		Version: d.Metadata.ConfiguredVersion,
		Kind:    "Descriptor",
	}
}

func (d *Descriptor) GetObjectKind() schema.ObjectKind {
	return d
}

func (d *Descriptor) DeepCopyObject() machineryruntime.Object {
	return &Descriptor{ComponentDescriptor: d.Copy()}
}

// Deserialize decodes the component descriptor from its serialized form.
func Deserialize(compdescBytes []byte) (*compdesc.ComponentDescriptor, error) {
	desc, err := compdesc.Decode(compdescBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	return desc, nil
}
