package manifest

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/orphan"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/security"
)

const controllerName = "manifest"

type SetupOptions struct {
	ListenerAddr                 string
	EnableDomainNameVerification bool
}

func SetupWithManager(mgr manager.Manager, opts ctrlruntime.Options, requeueIntervals queue.RequeueIntervals,
	settings SetupOptions, manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics, manifestClient declarativev2.ManifestAPIClient,
	orphanDetectionClient orphan.DetectionRepository, specResolver *spec.Resolver,
	clientCache declarativev2.SKRClientCache,
	clientFactory declarativev2.SKRClientFactory,
	accessManagerService *accessmanager.Service,
) error {
	var verifyFunc watcherevent.Verify
	if settings.EnableDomainNameVerification {
		// Verifier used to verify incoming listener requests
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}

	runnableListener := watcherevent.NewSKREventListener(
		settings.ListenerAddr, strings.ToLower(shared.OperatorName),
		verifyFunc,
	)

	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
		return fmt.Errorf("failed to add to listener to manager: %w", err)
	}

	addSkrEventToQueueFunc := &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
			queue workqueue.TypedRateLimitingInterface[ctrl.Request],
		) {
			ctrl.Log.WithName("listener").Info(
				fmt.Sprintf(
					"event coming from SKR, adding %s to queue",
					client.ObjectKeyFromObject(evnt.Object).String(),
				),
			)
			queue.Add(ctrl.Request{NamespacedName: client.ObjectKeyFromObject(evnt.Object)})
		},
	}

	skrEventChannel := source.Channel(runnableListener.ReceivedEvents, addSkrEventToQueueFunc)
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Named(controllerName).
		Watches(&apicorev1.Secret{}, handler.Funcs{},
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{},
				predicate.LabelChangedPredicate{}))).
		WatchesRawSource(skrEventChannel).
		WithOptions(opts).
		Complete(NewReconciler(mgr, requeueIntervals, manifestMetrics, mandatoryModulesMetrics,
			manifestClient, orphanDetectionClient, specResolver, clientCache, clientFactory,
			accessManagerService)); err != nil {
		return fmt.Errorf("failed to setup manager for manifest controller: %w", err)
	}

	return nil
}
