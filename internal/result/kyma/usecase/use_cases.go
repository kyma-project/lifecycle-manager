package usecase

import "github.com/kyma-project/lifecycle-manager/internal/result"

const (
	UseCaseSetKcpKymaStateDeleting  result.UseCase = "SetKcpKymaStateDeleting"
	UseCaseSetSkrKymaStateDeleting  result.UseCase = "SetSkrKymaStateDeleting"
	UseCaseDeleteSkrKyma            result.UseCase = "DeleteSkrKyma"
	UseCaseDeleteSkrWatcher         result.UseCase = "DeleteSkrWebhook"
	UseCaseDeleteSkrModuleMetadata  result.UseCase = "DeleteSkrModuleMetadata"
	UseCaseDeleteSkrCrds            result.UseCase = "DeleteSkrCrds"
	UseCaseDeleteWatcherCertificate result.UseCase = "DeleteWatcherCertificate"
	UseCaseDeleteManifests          result.UseCase = "DeleteManifests"
	UseCaseDeleteMetrics            result.UseCase = "DeleteMetrics"
	UseCaseRemoveKymaFinalizers     result.UseCase = "RemoveKymaFinalizers"
)
