package provider

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrGettingObject = errors.New("failed to get object")

type ObjectProvider struct {
	Client           client.Reader
	Name             k8stypes.NamespacedName
	GroupVersionKind schema.GroupVersionKind
}

func (p ObjectProvider) Get(ctx context.Context) (*unstructured.Unstructured, error) {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(p.GroupVersionKind)

	if err := p.Client.Get(ctx, p.Name, object); err != nil {
		return nil, fmt.Errorf("%w %v/%v in %v: %w", ErrGettingObject, p.GroupVersionKind.Kind, p.Name.Name, p.Name.Namespace, err)
	}

	return object, nil
}
