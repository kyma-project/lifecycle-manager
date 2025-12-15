package deletion

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	kymastatusrepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma/status"
	manifestrepo "github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	skrcrdrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/crd"
	skrkymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma"
	skrkymastatusrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma/status"
	"github.com/kyma-project/lifecycle-manager/internal/repository/skr/webhook"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	kymadeletionsvc "github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

//nolint:funlen // composition function
func ComposeKymaDeletionService(kcpClient client.Client,
	certificateRepository certificate.CertificateRepository,
	kymaMetrics *metrics.KymaMetrics,
	kymaRepo *kymarepo.Repository,
	accessSecretRepository *secretrepo.Repository,
	skrClientCache *remote.ClientCache,
	skrWebhookManager *watcher.SkrWebhookManifestManager,
) *kymadeletionsvc.Service {
	kymaStatusRepo := kymastatusrepo.NewRepository(kcpClient.Status())
	setKcpKymaStateDeleting := usecases.NewSetKymaStatusDeletingUseCase(kymaStatusRepo)

	// Create the SKR client retriever function from the same cache instance
	// This serves only as an adapter until we have a better way of managing SKR clients
	// See issue: https://github.com/kyma-project/lifecycle-manager/issues/2888
	skrClientRetrieverFunc := func(kymaName types.NamespacedName) (client.Client, error) {
		skrClient := skrClientCache.Get(kymaName)
		if skrClient == nil {
			return nil, fmt.Errorf("%w: Kyma %s", errorsinternal.ErrSkrClientNotFound, kymaName.String())
		}
		return skrClient, nil
	}

	setSkrKymaStateDeleting := usecases.NewSetSkrKymaStateDeleting(
		skrkymastatusrepo.NewRepository(skrClientRetrieverFunc),
		accessSecretRepository,
	)

	deleteSkrKyma := usecases.NewDeleteSkrKyma(
		skrkymarepo.NewRepository(skrClientRetrieverFunc),
		accessSecretRepository,
	)

	istioSystemSecretRepo := secretrepo.NewRepository(
		kcpClient,
		shared.IstioNamespace,
	)
	deleteWatcherCertificateSetup := usecases.NewDeleteWatcherCertificateSetup(
		certificateRepository,
		istioSystemSecretRepo,
	)

	skrWebhookResourcesRepo := webhook.NewResourceRepository(skrClientRetrieverFunc, shared.DefaultRemoteNamespace,
		skrWebhookManager.BaseResources)
	deleteSkrWebhookResources := usecases.NewDeleteSkrWebhookResources(skrWebhookResourcesRepo)

	deleteSkrMtCrd := usecases.NewDeleteSkrCrd(
		skrcrdrepo.NewRepository(skrClientRetrieverFunc,
			fmt.Sprintf("%s.%s", shared.ModuleTemplateKind.Plural(), shared.OperatorGroup)),
		accessSecretRepository,
		usecase.DeleteSkrModuleTemplateCrd)
	deleteSkrMrmCrd := usecases.NewDeleteSkrCrd(
		skrcrdrepo.NewRepository(skrClientRetrieverFunc,
			fmt.Sprintf("%s.%s", shared.ModuleReleaseMetaKind.Plural(), shared.OperatorGroup)),
		accessSecretRepository,
		usecase.DeleteSkrModuleReleaseMetaCrd)
	deleteSkrKymaCrd := usecases.NewDeleteSkrCrd(
		skrcrdrepo.NewRepository(skrClientRetrieverFunc,
			fmt.Sprintf("%s.%s", shared.KymaKind.Plural(), shared.OperatorGroup)),
		accessSecretRepository,
		usecase.DeleteSkrKymaCrd)

	deleteManifests := usecases.NewDeleteManifests(
		manifestrepo.NewRepository(
			kcpClient,
			shared.DefaultControlPlaneNamespace),
	)

	deleteMetrics := usecases.NewDeleteMetrics(kymaMetrics)

	dropKymaFinalizer := usecases.NewDropKymaFinalizer(kymaRepo)

	svc, err := kymadeletionsvc.NewService(
		setKcpKymaStateDeleting,
		setSkrKymaStateDeleting,
		deleteSkrKyma,
		deleteWatcherCertificateSetup,
		deleteSkrWebhookResources,
		deleteSkrMtCrd,
		deleteSkrMrmCrd,
		deleteSkrKymaCrd,
		deleteManifests,
		deleteMetrics,
		dropKymaFinalizer,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to compose Kyma deletion service: %v", err))
	}

	return svc
}
