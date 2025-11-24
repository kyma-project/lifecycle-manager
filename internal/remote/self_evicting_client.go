package remote

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SelfEvictingClient struct {
	clnt       client.Client
	statusClnt client.SubResourceWriter
	evict      func()
}

func NewSelfEvictingClient(clnt client.Client, evict func()) *SelfEvictingClient {
	return &SelfEvictingClient{
		clnt: clnt,
		statusClnt: &selfEvictingStatusClient{
			baseStatusClient: clnt.Status(),
			evict:            evict,
		},
		evict: evict,
	}
}

func (c *SelfEvictingClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.clnt.GroupVersionKindFor(obj)
}

func (c *SelfEvictingClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.clnt.IsObjectNamespaced(obj)
}

func (c *SelfEvictingClient) Scheme() *runtime.Scheme {
	return c.clnt.Scheme()
}

func (c *SelfEvictingClient) RESTMapper() meta.RESTMapper {
	return c.clnt.RESTMapper()
}

func (c *SelfEvictingClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	err := c.clnt.Apply(ctx, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *SelfEvictingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	err := c.clnt.Create(ctx, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *SelfEvictingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	err := c.clnt.Delete(ctx, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *SelfEvictingClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	err := c.clnt.DeleteAllOf(ctx, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *SelfEvictingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := c.clnt.Get(ctx, key, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *SelfEvictingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := c.clnt.List(ctx, list, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *SelfEvictingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	err := c.clnt.Patch(ctx, obj, patch, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *SelfEvictingClient) Status() client.SubResourceWriter {
	return c.statusClnt
}

func (c *SelfEvictingClient) SubResource(subResource string) client.SubResourceClient {
	return &selfEvictingSubResourceClient{
		baseSubResourceClient: c.clnt.SubResource(subResource),
		evict:                 c.evict,
	}
}

func (c *SelfEvictingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	err := c.clnt.Update(ctx, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

type selfEvictingStatusClient struct {
	baseStatusClient client.SubResourceWriter
	evict            func()
}

func (c *selfEvictingStatusClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	err := c.baseStatusClient.Create(ctx, obj, subResource, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *selfEvictingStatusClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	err := c.baseStatusClient.Update(ctx, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *selfEvictingStatusClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	err := c.baseStatusClient.Patch(ctx, obj, patch, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

type selfEvictingSubResourceClient struct {
	baseSubResourceClient client.SubResourceClient
	evict                 func()
}

func (c *selfEvictingSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	err := c.baseSubResourceClient.Create(ctx, obj, subResource, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *selfEvictingSubResourceClient) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	err := c.baseSubResourceClient.Get(ctx, obj, subResource, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *selfEvictingSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	err := c.baseSubResourceClient.Patch(ctx, obj, patch, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}

func (c *selfEvictingSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	err := c.baseSubResourceClient.Update(ctx, obj, opts...)
	if util.IsConnectionRelatedError(err) {
		c.evict()
	}
	return err
}
