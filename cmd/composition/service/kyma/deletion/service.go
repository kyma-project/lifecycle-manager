package deletion

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	setSkrKymaStateDeleting := usecases.NewSetSkrKymaStateDeleting(
		skrkymastatusrepo.NewRepository(skrClientCache),
		accessSecretRepository,
	)

	deleteSkrKyma := usecases.NewDeleteSkrKyma(
		skrkymarepo.NewRepository(skrClientCache),
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

	skrWebhookResourcesRepo := webhook.NewResourceRepository(skrClientCache, shared.DefaultRemoteNamespace,
		skrWebhookManager.BaseResources)
	deleteSkrWebhookResources := usecases.NewDeleteSkrWebhookResources(skrWebhookResourcesRepo)

	deleteSkrMtCrd := usecases.NewDeleteSkrCrd(
		skrcrdrepo.NewRepository(skrClientCache,
			fmt.Sprintf("%s.%s", shared.ModuleTemplateKind.Plural(), shared.OperatorGroup)),
		accessSecretRepository,
		usecase.DeleteSkrModuleTemplateCrd)
	deleteSkrMrmCrd := usecases.NewDeleteSkrCrd(
		skrcrdrepo.NewRepository(skrClientCache,
			fmt.Sprintf("%s.%s", shared.ModuleReleaseMetaKind.Plural(), shared.OperatorGroup)),
		accessSecretRepository,
		usecase.DeleteSkrModuleReleaseMetaCrd)
	deleteSkrKymaCrd := usecases.NewDeleteSkrCrd(
		skrcrdrepo.NewRepository(skrClientCache,
			fmt.Sprintf("%s.%s", shared.KymaKind.Plural(), shared.OperatorGroup)),
		accessSecretRepository,
		usecase.DeleteSkrKymaCrd)

	deleteManifests := usecases.NewDeleteManifests(
		manifestrepo.NewRepository(
			kcpClient,
			shared.DefaultControlPlaneNamespace),
	)

	deleteMetrics := usecases.NewDeleteMetrics(kymaMetrics)

	dropKymaFinalizers := usecases.NewDropKymaFinalizers(kymaRepo)

	return kymadeletionsvc.NewService(
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
		dropKymaFinalizers,
	)
}
