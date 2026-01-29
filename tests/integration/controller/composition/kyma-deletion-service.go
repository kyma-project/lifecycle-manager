package composition

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	kymadeletioncmpse "github.com/kyma-project/lifecycle-manager/cmd/composition/service/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/cmd/composition/service/skrwebhook"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	kymadeletionsvc "github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

func ComposeKymaDeletionService(
	kcpClient client.Client,
	skrClientCache *remote.ClientCache,
	skrWebhookManager *watcher.SkrWebhookManifestManager,
	certificateRepository skrwebhook.CertificateRepository,
	kymaMetrics *metrics.KymaMetrics,
	testEventRec *event.RecorderWrapper,
	flagVar *flags.FlagVar,
) *kymadeletionsvc.Service {
	kymaRepo := kymarepo.NewRepository(kcpClient, shared.DefaultControlPlaneNamespace)
	accessSecretRepository := secretrepo.NewRepository(kcpClient, shared.DefaultControlPlaneNamespace)

	return kymadeletioncmpse.ComposeKymaDeletionService(
		kcpClient,
		certificateRepository,
		kymaMetrics,
		kymaRepo,
		accessSecretRepository,
		skrClientCache,
		skrWebhookManager,
	)
}
