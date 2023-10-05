package v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/internal"
)

var ErrClientObjectConversionFailed = errors.New("client object conversion failed")

type SSA interface {
	Run(context.Context, []*resource.Info) error
}

type ConcurrentDefaultSSA struct {
	clnt      client.Client
	owner     client.FieldOwner
	versioner runtime.GroupVersioner
	converter runtime.ObjectConvertor
}

func ConcurrentSSA(clnt client.Client, owner client.FieldOwner) *ConcurrentDefaultSSA {
	return &ConcurrentDefaultSSA{
		clnt: clnt, owner: owner,
		versioner: schema.GroupVersions(clnt.Scheme().PrioritizedVersionsAllGroups()),
		converter: clnt.Scheme(),
	}
}

func (c *ConcurrentDefaultSSA) Run(ctx context.Context, resources []*resource.Info) error {
	ssaStart := time.Now()
	logger := log.FromContext(ctx, "owner", c.owner)
	logger.V(internal.TraceLogLevel).Info("ServerSideApply", "resources", len(resources))

	// The Runtime Complexity of this Branch is N as only ServerSideApplier Patch is required
	results := make(chan error, len(resources))
	for i := range resources {
		i := i
		go c.serverSideApply(ctx, resources[i], results)
	}

	var errs []error
	for i := 0; i < len(resources); i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	ssaFinish := time.Since(ssaStart)

	if errs != nil {
		errs = append(errs, fmt.Errorf("ServerSideApply failed (after %s)", ssaFinish))
		return errors.Join(errs...)
	}
	logger.V(internal.DebugLogLevel).Info("ServerSideApply finished", "time", ssaFinish)
	return nil
}

func (c *ConcurrentDefaultSSA) serverSideApply(
	ctx context.Context,
	resource *resource.Info,
	results chan error,
) {
	start := time.Now()
	logger := log.FromContext(ctx, "owner", c.owner)

	// this converts unstructured to typed objects if possible, leveraging native APIs
	resource.Object = c.convertUnstructuredToTyped(resource.Object, resource.Mapping)

	logger.V(internal.TraceLogLevel).Info(
		fmt.Sprintf("apply %s", resource.ObjectName()),
	)

	results <- c.serverSideApplyResourceInfo(ctx, resource)

	logger.V(internal.TraceLogLevel).Info(
		fmt.Sprintf("apply %s finished", resource.ObjectName()),
		"time", time.Since(start),
	)
}

func (c *ConcurrentDefaultSSA) serverSideApplyResourceInfo(
	ctx context.Context,
	info *resource.Info,
) error {
	obj, isTyped := info.Object.(client.Object)
	if !isTyped {
		return fmt.Errorf(
			"%s is not a valid client-go object: %w", info.ObjectName(), ErrClientObjectConversionFailed,
		)
	}
	obj.SetManagedFields(nil)
	err := c.clnt.Patch(ctx, obj, client.Apply, client.ForceOwnership, c.owner)
	if err != nil {
		return fmt.Errorf(
			"patch for %s failed: %w", info.ObjectName(), err,
		)
	}

	return nil
}

// convertWithMapper converts the given object with the optional provided
// RESTMapping. If no mapping is provided, the default schema versioner is used.
//

func (c *ConcurrentDefaultSSA) convertUnstructuredToTyped(
	obj runtime.Object, mapping *meta.RESTMapping,
) runtime.Object {
	gv := c.versioner
	if mapping != nil {
		gv = mapping.GroupVersionKind.GroupVersion()
	}
	if obj, err := c.converter.ConvertToVersion(obj, gv); err == nil {
		return obj
	}
	return obj
}
