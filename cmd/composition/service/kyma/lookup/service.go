package service

import (
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	kymalookupsvc "github.com/kyma-project/lifecycle-manager/internal/service/kyma/lookup"
)

func ComposeKymaLookupService(kymaRepo *kymarepo.Repository) *kymalookupsvc.Service {
	return kymalookupsvc.NewService(kymaRepo)
}
