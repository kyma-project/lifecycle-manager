package manifest

import (
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

const controllerName = "manifest"

type SetupOptions struct {
	ListenerAddr string
}

func SetupWithManager(mgr manager.Manager,
	opts ctrlruntime.Options,
	requeueIntervals queue.RequeueIntervals,
	rateLimiter workqueue.TypedRateLimiter[ctrl.Request],
	manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestClient declarativev2.ManifestAPIClient,
	orphanDetectionService declarativev2.OrphanDetectionService,
	specResolver *spec.Resolver,
	skrClientCache declarativev2.SKRClientCache,
	skrClient declarativev2.SKRClient,
	kcpClient client.Client,
	cachedManifestParser declarativev2.CachedManifestParser,
	customStateCheck declarativev2.StateCheck,
	skrImagePullSecretName string,
) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Named(controllerName).
		Watches(&apicorev1.Secret{}, handler.Funcs{},
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{},
				predicate.LabelChangedPredicate{}))).
		WithOptions(opts).
		Complete(declarativev2.NewReconciler(
			requeueIntervals, rateLimiter, manifestMetrics, mandatoryModulesMetrics, manifestClient,
			orphanDetectionService, specResolver, skrClientCache, skrClient, kcpClient, cachedManifestParser,
			customStateCheck, skrImagePullSecretName)); err != nil {
		return fmt.Errorf("failed to setup manager for manifest controller: %w", err)
	}

	return nil
}
