package ownerlookup

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrLookingUpOwner = errors.New("failed to lookup owner")
)

type OwnerLookup struct {
	Client           client.Reader
	Name             types.NamespacedName
	GroupVersionKind schema.GroupVersionKind
}

func (ol OwnerLookup) GetOwner(ctx context.Context) (*unstructured.Unstructured, error) {
	owner := &unstructured.Unstructured{}
	owner.SetGroupVersionKind(ol.GroupVersionKind)

	if err := ol.Client.Get(ctx, ol.Name, owner); err != nil {
		return nil, fmt.Errorf("%w %v/%v in %v: %v", ErrLookingUpOwner, ol.GroupVersionKind.Kind, ol.Name.Name, ol.Name.Namespace, err)
	}

	return owner, nil
}
