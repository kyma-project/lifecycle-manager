package v2

import (
	"context"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrDeletionNotFinished = errors.New("deletion is not yet finished")

type Cleanup interface {
	Run(ctx context.Context, resourceInfo []*resource.Info) error
}

type ConcurrentCleanup struct {
	clnt   client.Client
	policy client.PropagationPolicy
}

func NewConcurrentCleanup(clnt client.Client) Cleanup {
	return &ConcurrentCleanup{clnt: clnt, policy: client.PropagationPolicy(metav1.DeletePropagationBackground)}
}

func (c *ConcurrentCleanup) Run(ctx context.Context, infos []*resource.Info) error {
	// The Runtime Complexity of this Branch is N as only ServerSideApplier Patch is required
	results := make(chan error, len(infos))
	for i := range infos {
		i := i
		go c.cleanupResource(ctx, infos[i], results)
	}

	var errs []error
	present := len(infos)
	for i := 0; i < len(infos); i++ {
		err := <-results
		if util.IsNotFound(err) {
			present--
			continue
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if present > 0 {
		return ErrDeletionNotFinished
	}
	return nil
}

func (c *ConcurrentCleanup) cleanupResource(ctx context.Context, info *resource.Info, results chan error) {
	obj, ok := info.Object.(client.Object)
	if !ok {
		return
	}
	results <- c.clnt.Delete(ctx, obj, c.policy)
}
