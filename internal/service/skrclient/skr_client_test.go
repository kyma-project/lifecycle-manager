package skrclient_test

import (
	"context"

	"k8s.io/client-go/rest"
)

type FakeAccessManagerService struct{}

func (f *FakeAccessManagerService) GetAccessRestConfigByKyma(_ context.Context, _ string) (*rest.Config, error) {
	return &rest.Config{Host: "http://example.invalid"}, nil
}
