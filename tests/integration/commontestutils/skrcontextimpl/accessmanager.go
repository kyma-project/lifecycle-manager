package skrcontextimpl

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type FakeAccessManagerService struct {
	skrEnv     *envtest.Environment
	baseConfig *rest.Config
}

func NewFakeAccessManagerService(skrEnv *envtest.Environment, baseConfig *rest.Config) *FakeAccessManagerService {
	return &FakeAccessManagerService{
		skrEnv:     skrEnv,
		baseConfig: baseConfig,
	}
}

func (f *FakeAccessManagerService) GetAccessRestConfigByKyma(_ context.Context, _ string) (*rest.Config, error) {
	authUser, err := f.skrEnv.AddUser(
		envtest.User{
			Name:   "skr-admin-account",
			Groups: []string{"system:masters"},
		}, f.baseConfig,
	)
	return authUser.Config(), err
}
