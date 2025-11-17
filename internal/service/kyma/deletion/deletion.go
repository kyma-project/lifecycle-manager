package deletion

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
)

type Service struct{}

func (s *Service) Delete(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	return result.Result{
		UseCase: "NotImplementedYet",
		Err:     errors.New("deletion service is not implemented yet"),
	}
}
