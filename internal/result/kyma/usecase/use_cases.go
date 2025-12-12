package usecase

import "github.com/kyma-project/lifecycle-manager/internal/result"

const (
	SetKcpKymaStateDeleting       result.UseCase = "SetKcpKymaStateDeleting"
	SetSkrKymaStateDeleting       result.UseCase = "SetSkrKymaStateDeleting"
	DeleteSkrKyma                 result.UseCase = "DeleteSkrKyma"
	DeleteWatcherCertificateSetup result.UseCase = "DeleteCertificateSetup"
	DeleteSkrWebhookResources     result.UseCase = "DeleteSkrWebhookResources"
	DeleteSkrModuleTemplateCrd    result.UseCase = "DeleteSkrModuleTemplateCrd"
	DeleteSkrModuleReleaseMetaCrd result.UseCase = "DeleteSkrModuleReleaseMetaCrd"
	DeleteSkrKymaCrd              result.UseCase = "DeleteSkrKymaCrd"
	DeleteManifests               result.UseCase = "DeleteManifests"
	DeleteMetrics                 result.UseCase = "DeleteMetrics"
	DropKymaFinalizer             result.UseCase = "DropKymaFinalizer"
)
