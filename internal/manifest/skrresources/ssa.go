package skrresources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/internal"
)

var (
	ErrClientObjectConversionFailed = errors.New("client object conversion failed")
	ErrServerSideApplyFailed        = errors.New("ServerSideApply failed")
	ErrClientUnauthorized           = errors.New("ServerSideApply is unauthorized")
)

type SSA interface {
	Run(ctx context.Context, resourceInfo []*resource.Info) error
}

type ManagedFieldsCollector interface {
	// Collect collects managed fields data from the single object
	Collect(ctx context.Context, obj client.Object)
	// Emit emits collected data to some backing store
	Emit(ctx context.Context) error
}

type ConcurrentDefaultSSA struct {
	clnt      client.Client
	owner     client.FieldOwner
	versioner machineryruntime.GroupVersioner
	converter machineryruntime.ObjectConvertor
	collector ManagedFieldsCollector
}

func ConcurrentSSA(clnt client.Client, owner client.FieldOwner, managedFieldsCollector ManagedFieldsCollector) *ConcurrentDefaultSSA {
	return &ConcurrentDefaultSSA{
		clnt:      clnt,
		owner:     owner,
		versioner: schema.GroupVersions(clnt.Scheme().PrioritizedVersionsAllGroups()),
		converter: clnt.Scheme(),
		collector: managedFieldsCollector,
	}
}

func (c *ConcurrentDefaultSSA) Run(ctx context.Context, resources []*resource.Info) error {
	ssaStart := time.Now()
	logger := logf.FromContext(ctx, "owner", c.owner)
	logger.V(internal.TraceLogLevel).Info("ServerSideApply", "resources", len(resources))

	// The Runtime Complexity of this Branch is N as only ServerSideApplier Patch is required
	results := make(chan error, len(resources))
	for i := range resources {
		go c.serverSideApply(ctx, resources[i], results)
	}

	var errs []error
	for range resources {
		err := <-results
		if err != nil {
			errs = append(errs, err)
		}
	}

	ssaFinish := time.Since(ssaStart)

	if errs != nil {
		summaryErr := fmt.Errorf("%w (after %s)", ErrServerSideApplyFailed, ssaFinish)
		if c.allUnauthorized(errs) {
			return errors.Join(ErrClientUnauthorized, summaryErr)
		}
		errs = append(errs, summaryErr)
		return errors.Join(errs...)
	}
	logger.V(internal.DebugLogLevel).Info("ServerSideApply finished", "time", ssaFinish)
	err := c.collector.Emit(ctx)
	if err != nil {
		logger.V(internal.DebugLogLevel).Error(err, "error emitting data of unknown field managers")
	}
	return nil
}

func (c *ConcurrentDefaultSSA) allUnauthorized(errs []error) bool {
	errorCount := len(errs)
	if errorCount == 0 {
		return false
	}

	unauthorizedFound := 0
	for i := range errs {
		if errors.Is(errs[i], ErrClientUnauthorized) {
			unauthorizedFound++
		}
	}

	return unauthorizedFound == errorCount
}

func (c *ConcurrentDefaultSSA) serverSideApply(
	ctx context.Context,
	resource *resource.Info,
	results chan error,
) {
	start := time.Now()
	logger := logf.FromContext(ctx, "owner", c.owner)
	logger.V(internal.TraceLogLevel).Info("apply " + resource.ObjectName())
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
			"patch for %s failed: %w", info.ObjectName(), c.suppressUnauthorized(err),
		)
	}

	c.collector.Collect(ctx, obj)
	return nil
}

// suppressUnauthorized replaces client-go error with our own in order to suppress it's very long Error() payload.
func (c *ConcurrentDefaultSSA) suppressUnauthorized(src error) error {
	if strings.HasSuffix(strings.TrimRight(src.Error(), " \n"), ": Unauthorized") {
		return ErrClientUnauthorized
	}
	return src
}
