package manifest

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

func NewReconciler(mgr manager.Manager,
	requeueIntervals queue.RequeueIntervals,
	manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestClient declarativev2.ManifestAPIClient,
	orphanDetectionService declarativev2.OrphanDetectionService,
	specResolver *spec.Resolver,
	skrClientCache declarativev2.SKRClientCache,
	skrClient declarativev2.SKRClient,
) *declarativev2.Reconciler {
	statefulChecker := statecheck.NewStatefulSetStateCheck()
	deploymentChecker := statecheck.NewDeploymentStateCheck()
	return declarativev2.NewFromManager(
		mgr, requeueIntervals, manifestMetrics, mandatoryModulesMetrics, manifestClient, orphanDetectionService,
		specResolver,
		skrClientCache,
		skrClient,
		statecheck.NewManagerStateCheck(statefulChecker, deploymentChecker),
	)
}
