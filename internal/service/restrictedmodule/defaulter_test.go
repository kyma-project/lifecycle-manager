package restrictedmodule_test

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/restrictedmodule"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	module1                  = random.Name()
	module2                  = random.Name()
	restrictedDefaultModules = []string{module1, module2}
)

func Test_Default_Skips_WhenKymaIsBeingDeleted(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	now := metav1.NewTime(time.Now())
	kyma.SetDeletionTimestamp(&now)

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		positiveMatchFunc)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.False(t, mrmRepo.called)
	assert.False(t, kymaRepo.called)
}

func Test_Default_Skips_WhenNoRestrictedDefaultModules(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter([]string{},
		mrmRepo,
		kymaRepo,
		positiveMatchFunc)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.False(t, mrmRepo.called)
	assert.False(t, kymaRepo.called)
}

func Test_Default_Skips_WhenAlreadyEnabled(t *testing.T) {
	kyma := &v1beta2.Kyma{
		Spec: v1beta2.KymaSpec{
			Modules: []v1beta2.Module{
				{Name: module1},
				{Name: module2},
			},
		},
	}

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		positiveMatchFunc)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.False(t, mrmRepo.called)
	assert.False(t, kymaRepo.called)
}

func Test_Defualt_Skips_WhenFailedToGetModuleReleaseMeta(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	mrmRepo := &mrmStub{
		err: assert.AnError,
	}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		positiveMatchFunc)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.True(t, mrmRepo.called)
	assert.ElementsMatch(t, mrmRepo.moduleNames, restrictedDefaultModules)
	assert.False(t, kymaRepo.called)
}

func Test_Default_Skips_WhenMatchFuncReturnsError(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		errorMatchFunc,
	)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.True(t, mrmRepo.called)
	assert.ElementsMatch(t, mrmRepo.moduleNames, restrictedDefaultModules)
	assert.False(t, kymaRepo.called)
}

func Test_Default_Skips_WhenMatchFuncReturnsFalse(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		negativeMatchFunc,
	)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.True(t, mrmRepo.called)
	assert.ElementsMatch(t, mrmRepo.moduleNames, restrictedDefaultModules)
	assert.False(t, kymaRepo.called)
}

func Test_Default_ReturnsError_WhenFailedToUpdateKyma(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{
		err: assert.AnError,
	}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		positiveMatchFunc,
	)

	err := defaulter.Default(context.Background(), kyma)

	require.Error(t, err)
	assert.True(t, mrmRepo.called)
	assert.ElementsMatch(t, mrmRepo.moduleNames, restrictedDefaultModules)
	assert.True(t, kymaRepo.called)
	assert.Equal(t, kyma, kymaRepo.kyma)
}

func Test_Default_AppendsModulesAndUpdatesKyma(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		positiveMatchFunc,
	)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.True(t, mrmRepo.called)
	assert.ElementsMatch(t, mrmRepo.moduleNames, restrictedDefaultModules)
	assert.True(t, kymaRepo.called)
	assert.Len(t, kymaRepo.kyma.Spec.Modules, len(restrictedDefaultModules))
	for _, module := range kymaRepo.kyma.Spec.Modules {
		assert.Contains(t, restrictedDefaultModules, module.Name)
		assert.True(t, module.Managed)
		assert.Equal(t, v1beta2.CustomResourcePolicyCreateAndDelete, string(module.CustomResourcePolicy))
	}
}

func Test_Defualt_AppendsOnlyMissingModules(t *testing.T) {
	kyma := &v1beta2.Kyma{
		Spec: v1beta2.KymaSpec{
			Modules: []v1beta2.Module{
				{
					Name:                 module1,
					Managed:              true,
					CustomResourcePolicy: v1beta2.CustomResourcePolicyCreateAndDelete,
				},
			},
		},
	}

	mrmRepo := &mrmStub{}
	kymaRepo := &kymaStub{}

	defaulter := restrictedmodule.NewDefaulter(restrictedDefaultModules,
		mrmRepo,
		kymaRepo,
		positiveMatchFunc,
	)

	err := defaulter.Default(context.Background(), kyma)

	require.NoError(t, err)
	assert.True(t, mrmRepo.called)
	assert.ElementsMatch(t, mrmRepo.moduleNames, []string{module2})
	assert.True(t, kymaRepo.called)
	assert.Len(t, kymaRepo.kyma.Spec.Modules, len(restrictedDefaultModules))
	for _, module := range kymaRepo.kyma.Spec.Modules {
		assert.Contains(t, restrictedDefaultModules, module.Name)
		assert.True(t, module.Managed)
		assert.Equal(t, v1beta2.CustomResourcePolicyCreateAndDelete, string(module.CustomResourcePolicy))
	}
}

func positiveMatchFunc(_ *v1beta2.ModuleReleaseMeta, _ *v1beta2.Kyma) (bool, error) {
	return true, nil
}

func negativeMatchFunc(_ *v1beta2.ModuleReleaseMeta, _ *v1beta2.Kyma) (bool, error) {
	return false, nil
}

func errorMatchFunc(_ *v1beta2.ModuleReleaseMeta, _ *v1beta2.Kyma) (bool, error) {
	return false, assert.AnError
}

type mrmStub struct {
	restrictedmodule.ModuleReleaseMetaRepository

	called      bool
	moduleNames []string
	mrm         *v1beta2.ModuleReleaseMeta
	err         error
}

func (r *mrmStub) Get(_ context.Context, moduleName string) (*v1beta2.ModuleReleaseMeta, error) {
	r.called = true
	r.moduleNames = append(r.moduleNames, moduleName)
	return r.mrm, r.err
}

type kymaStub struct {
	restrictedmodule.KymaRepository

	called bool
	kyma   *v1beta2.Kyma
	err    error
}

func (r *kymaStub) Update(_ context.Context, kyma *v1beta2.Kyma) error {
	r.called = true
	r.kyma = kyma
	return r.err
}
