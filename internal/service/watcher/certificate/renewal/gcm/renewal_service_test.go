package gcm_test

import (
	"context"
	"testing"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal/gcm"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRenew_WhenRepoReturnsError_ReturnsError(t *testing.T) {
	service := gcm.NewService(nil)

	_ = service.Renew(t.Context(), random.Name())
}

type certRepoStub struct {
	getCalled       bool
	getCallerArg    string
	updateCalled    bool
	updateCallerArg *gcertv1alpha1.Certificate
	err             error
}

func (c *certRepoStub) Get(ctx context.Context, name string) (*gcertv1alpha1.Certificate, error) {
	c.getCalled = true
	c.getCallerArg = name
	return nil, c.err
}

func (c *certRepoStub) Update(ctx context.Context, cert *gcertv1alpha1.Certificate) error {
	c.updateCalled = true
	c.updateCallerArg = cert
	return nil
}
