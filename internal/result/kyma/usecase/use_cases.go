package usecase

import "github.com/kyma-project/lifecycle-manager/internal/result"

const (
	SetKcpKymaStateDeleting       result.UseCase = "SetKcpKymaStateDeleting"
	SetSkrKymaStateDeleting       result.UseCase = "SetSkrKymaStateDeleting"
	DeleteSkrKyma                 result.UseCase = "DeleteSkrKyma"
	DeleteSkrWatcher              result.UseCase = "DeleteSkrWebhook"
	DeleteSkrModuleTemplateCrd    result.UseCase = "DeleteSkrModuleTemplateCrd"
	DeleteSkrModuleReleaseMetaCrd result.UseCase = "DeleteSkrModuleReleaseMetaCrd"
	DeleteSkrKymaCrd              result.UseCase = "DeleteSkrKymaCrd"
	DeleteWatcherCertificate      result.UseCase = "DeleteWatcherCertificate"
	DeleteManifests               result.UseCase = "DeleteManifests"
	DeleteMetrics                 result.UseCase = "DeleteMetrics"
	RemoveKymaFinalizers          result.UseCase = "RemoveKymaFinalizers"
)
