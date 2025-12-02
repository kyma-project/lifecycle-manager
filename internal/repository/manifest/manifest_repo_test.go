package manifest_test

import (
	"context"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clientStub struct {
	client.Client

	deleteAllOfCalled bool
	listCalled        bool
	deleteAllOfErr    error
	listErr           error

	capturedNamespace  string
	capturedLabels     map[string]string
	capturedObjectType client.Object

	partialObjectMetadata []apimetav1.PartialObjectMetadata
}

func (c *clientStub) DeleteAllOf(_ context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	c.deleteAllOfCalled = true
	c.capturedObjectType = obj

	for _, opt := range opts {
		if nsOpt, ok := opt.(client.InNamespace); ok {
			c.capturedNamespace = string(nsOpt)
		}
		if labelOpt, ok := opt.(client.MatchingLabels); ok {
			c.capturedLabels = labelOpt
		}
	}

	return c.deleteAllOfErr
}

func (c *clientStub) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.listCalled = true

	for _, opt := range opts {
		if nsOpt, ok := opt.(client.InNamespace); ok {
			c.capturedNamespace = string(nsOpt)
		}
		if labelOpt, ok := opt.(client.MatchingLabels); ok {
			c.capturedLabels = labelOpt
		}
	}

	if c.listErr != nil {
		return c.listErr
	}

	if partialList, ok := list.(*apimetav1.PartialObjectMetadataList); ok {
		partialList.Items = c.partialObjectMetadata
	}

	return nil
}
