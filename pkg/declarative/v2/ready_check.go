package v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/pkg/types"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrResourcesNotReady           = errors.New("resources are not ready")
	ErrCustomResourceStateNotFound = errors.New("custom resource state not found")
	ErrManifestDeployNotReady = errors.New("manifest deployment is not ready")
)

type ReadyCheck interface {
	Run(ctx context.Context, clnt Client, obj Object, resources []*resource.Info) error
}

type HelmReadyCheck struct {
	clientSet kubernetes.Interface
}

func NewHelmReadyCheck(factory kube.Factory) ReadyCheck {
	clientSet, _ := factory.KubernetesClientSet()
	return &HelmReadyCheck{clientSet: clientSet}
}

func NewExistsReadyCheck() ReadyCheck {
	return &ExistsReadyCheck{}
}

func (c *HelmReadyCheck) Run(ctx context.Context, _ Client, _ Object, resources []*resource.Info) error {
	start := time.Now()
	logger := log.FromContext(ctx)
	logger.V(internal.TraceLogLevel).Info("ReadyCheck", "resources", len(resources))
	checker := kube.NewReadyChecker(
		c.clientSet, func(format string, args ...interface{}) {
			logger.V(internal.DebugLogLevel).Info(fmt.Sprintf(format, args...))
		}, kube.PausedAsReady(false), kube.CheckJobs(true),
	)

	readyCheckResults := make(chan error, len(resources))

	isReady := func(ctx context.Context, i int) {
		ready, err := checker.IsReady(ctx, resources[i])
		if !ready {
			readyCheckResults <- ErrResourcesNotReady
		} else {
			readyCheckResults <- err
		}
	}

	for i := range resources {
		i := i
		go isReady(ctx, i)
	}

	var errs []error
	for i := 0; i < len(resources); i++ {
		err := <-readyCheckResults
		if errors.Is(err, ErrResourcesNotReady) {
			return err
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return types.NewMultiError(errs)
	}

	logger.V(internal.DebugLogLevel).Info(
		"ReadyCheck finished",
		"resources", len(resources), "time", time.Since(start),
	)

	return nil
}

type ExistsReadyCheck struct{}

func (c *ExistsReadyCheck) Run(ctx context.Context, clnt Client, _ Object, resources []*resource.Info) error {
	for i := range resources {
		obj, ok := resources[i].Object.(client.Object)
		if !ok {
			//nolint:goerr113
			return errors.New("object in resource info is not a valid client object")
		}
		if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}
