package usecase

import "github.com/kyma-project/lifecycle-manager/internal/result"

const (
	SetKcpKymaStateDeleting  result.UseCase = "SetKcpKymaStateDeleting"
	SetSkrKymaStateDeleting  result.UseCase = "SetSkrKymaStateDeleting"
	DeleteSkrKyma            result.UseCase = "DeleteSkrKyma"
	DeleteSkrWatcher         result.UseCase = "DeleteSkrWebhook"
	DeleteSkrModuleMetadata  result.UseCase = "DeleteSkrModuleMetadata"
	DeleteSkrCrds            result.UseCase = "DeleteSkrCrds"
	DeleteWatcherCertificate result.UseCase = "DeleteWatcherCertificate"
	DeleteManifests          result.UseCase = "DeleteManifests"
	DeleteMetrics            result.UseCase = "DeleteMetrics"
	RemoveKymaFinalizers     result.UseCase = "RemoveKymaFinalizers"
	ProcessKymaDeletion      result.UseCase = "ProcessKymaDeletion"
)
