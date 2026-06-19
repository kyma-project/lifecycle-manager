package skrsync_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrsync"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestCrdSync_WhenAllCrdsApply_Succeeds(t *testing.T) {
	kcpRepo := &kcpCrdRepoStub{
		crdsByName: map[string]*apiextensionsv1.CustomResourceDefinition{
			crdName(shared.KymaKind):              kymaCrd(),
			crdName(shared.ModuleTemplateKind):    moduleTemplateCrd(),
			crdName(shared.ModuleReleaseMetaKind): moduleReleaseMetaCrd(),
		},
	}
	kymaApplier := &skrCrdApplierStub{}
	mtApplier := &skrCrdApplierStub{}
	mrmApplier := &skrCrdApplierStub{}

	svc := skrsync.NewCrdSyncService(kcpRepo, kymaApplier, mtApplier, mrmApplier)

	kyma := newKyma()
	err := svc.Sync(t.Context(), kyma)

	require.NoError(t, err)
	assert.Equal(t, []string{
		crdName(shared.KymaKind),
		crdName(shared.ModuleTemplateKind),
		crdName(shared.ModuleReleaseMetaKind),
	}, kcpRepo.requestedNames)
	assert.Equal(t, kyma.GetNamespacedName(), kymaApplier.kymaName)
	assert.Equal(t, kyma.GetNamespacedName(), mtApplier.kymaName)
	assert.Equal(t, kyma.GetNamespacedName(), mrmApplier.kymaName)
	assert.Equal(t, kymaCrd(), kymaApplier.crd)
	assert.Equal(t, moduleTemplateCrd(), mtApplier.crd)
	assert.Equal(t, moduleReleaseMetaCrd(), mrmApplier.crd)
}

func TestCrdSync_WhenKcpReadFails_AggregatesAndContinues(t *testing.T) {
	readErr := errors.New("kcp not reachable")
	kcpRepo := &kcpCrdRepoStub{
		crdsByName: map[string]*apiextensionsv1.CustomResourceDefinition{
			crdName(shared.ModuleTemplateKind):    moduleTemplateCrd(),
			crdName(shared.ModuleReleaseMetaKind): moduleReleaseMetaCrd(),
		},
		errsByName: map[string]error{
			crdName(shared.KymaKind): readErr,
		},
	}
	kymaApplier := &skrCrdApplierStub{}
	mtApplier := &skrCrdApplierStub{}
	mrmApplier := &skrCrdApplierStub{}

	svc := skrsync.NewCrdSyncService(kcpRepo, kymaApplier, mtApplier, mrmApplier)

	err := svc.Sync(t.Context(), newKyma())

	require.Error(t, err)
	require.ErrorIs(t, err, skrsync.ErrCrdSync)
	require.ErrorIs(t, err, readErr)
	assert.False(t, kymaApplier.called)
	assert.True(t, mtApplier.called)
	assert.True(t, mrmApplier.called)
}

func TestCrdSync_WhenApplyFails_AggregatesError(t *testing.T) {
	applyErr := errors.New("apply failed")
	kcpRepo := &kcpCrdRepoStub{
		crdsByName: map[string]*apiextensionsv1.CustomResourceDefinition{
			crdName(shared.KymaKind):              kymaCrd(),
			crdName(shared.ModuleTemplateKind):    moduleTemplateCrd(),
			crdName(shared.ModuleReleaseMetaKind): moduleReleaseMetaCrd(),
		},
	}
	kymaApplier := &skrCrdApplierStub{err: applyErr}
	mtApplier := &skrCrdApplierStub{}
	mrmApplier := &skrCrdApplierStub{}

	svc := skrsync.NewCrdSyncService(kcpRepo, kymaApplier, mtApplier, mrmApplier)

	err := svc.Sync(t.Context(), newKyma())

	require.Error(t, err)
	require.ErrorIs(t, err, skrsync.ErrCrdSync)
	require.ErrorIs(t, err, applyErr)
	assert.True(t, kymaApplier.called)
	assert.True(t, mtApplier.called)
	assert.True(t, mrmApplier.called)
}

func newKyma() *v1beta2.Kyma {
	return &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: shared.DefaultControlPlaneNamespace,
		},
	}
}

func crdName(kind shared.Kind) string {
	return kind.Plural() + "." + shared.OperatorGroup
}

func kymaCrd() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{ObjectMeta: apimetav1.ObjectMeta{Name: crdName(shared.KymaKind)}}
}

func moduleTemplateCrd() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: apimetav1.ObjectMeta{Name: crdName(shared.ModuleTemplateKind)},
	}
}

func moduleReleaseMetaCrd() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: apimetav1.ObjectMeta{Name: crdName(shared.ModuleReleaseMetaKind)},
	}
}

type kcpCrdRepoStub struct {
	crdsByName     map[string]*apiextensionsv1.CustomResourceDefinition
	errsByName     map[string]error
	requestedNames []string
}

func (s *kcpCrdRepoStub) Get(_ context.Context, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	s.requestedNames = append(s.requestedNames, name)
	if err, ok := s.errsByName[name]; ok {
		return nil, err
	}
	return s.crdsByName[name], nil
}

type skrCrdApplierStub struct {
	called   bool
	kymaName types.NamespacedName
	crd      *apiextensionsv1.CustomResourceDefinition
	err      error
}

func (s *skrCrdApplierStub) Apply(_ context.Context,
	kymaName types.NamespacedName,
	crd *apiextensionsv1.CustomResourceDefinition,
) error {
	s.called = true
	s.kymaName = kymaName
	s.crd = crd
	return s.err
}
