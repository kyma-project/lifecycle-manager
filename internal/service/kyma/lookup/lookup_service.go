package lookup

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var (
	ErrLookup        = errors.New("error looking up Kyma")
	ErrNotFound      = errors.New("no instances found")
	ErrMultipleFound = errors.New("multiple Kyma instances found")
)

type Repository interface {
	LookupByLabel(ctx context.Context, labelKey, labelValue string) (*v1beta2.KymaList, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) ByRuntimeID(ctx context.Context, runtimeID string) (*v1beta2.Kyma, error) {
	res, err := s.byRuntimeID(ctx, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("%w with runtimeID=%s: %w", ErrLookup, runtimeID, err)
	}
	return res, nil
}

func (s *Service) byRuntimeID(ctx context.Context, runtimeID string) (*v1beta2.Kyma, error) {
	kymaList, err := s.repo.LookupByLabel(ctx, shared.RuntimeIDLabel, runtimeID)
	if err != nil {
		return nil, err
	}
	if len(kymaList.Items) == 0 {
		return nil, ErrNotFound
	}
	if len(kymaList.Items) > 1 {
		return nil, ErrMultipleFound
	}

	return &kymaList.Items[0], nil
}
