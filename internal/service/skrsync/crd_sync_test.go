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

func TestSyncCRDs_WhenAllAppliersSucceed_ReturnsNoError(t *testing.T) {
	kymaApplier := &skrCrdApplierStub{}
	mtApplier := &skrCrdApplierStub{}
	mrmApplier := &skrCrdApplierStub{}

	kcpRepo := &kcpCrdReaderStub{
		crdsByName: map[string]*apiextensionsv1.CustomResourceDefinition{
			crdName(shared.KymaKind):              fakeCrd(shared.KymaKind),
			crdName(shared.ModuleTemplateKind):    fakeCrd(shared.ModuleTemplateKind),
			crdName(shared.ModuleReleaseMetaKind): fakeCrd(shared.ModuleReleaseMetaKind),
		},
	}

	svc := skrsync.NewService(kcpRepo, []skrsync.SkrCrdSyncEntry{
		{Kind: shared.KymaKind, Applier: kymaApplier},
		{Kind: shared.ModuleTemplateKind, Applier: mtApplier},
		{Kind: shared.ModuleReleaseMetaKind, Applier: mrmApplier},
	}, nil, nil, "")

	kyma := newKyma()
	err := svc.SyncCRDs(t.Context(), kyma)

	require.NoError(t, err)
	assert.Equal(t, []string{
		crdName(shared.KymaKind),
		crdName(shared.ModuleTemplateKind),
		crdName(shared.ModuleReleaseMetaKind),
	}, kcpRepo.requestedNames)
	assert.Equal(t, kyma.GetNamespacedName(), kymaApplier.kymaName)
	assert.Equal(t, kyma.GetNamespacedName(), mtApplier.kymaName)
	assert.Equal(t, kyma.GetNamespacedName(), mrmApplier.kymaName)
	assert.Equal(t, fakeCrd(shared.KymaKind), kymaApplier.crd)
	assert.Equal(t, fakeCrd(shared.ModuleTemplateKind), mtApplier.crd)
	assert.Equal(t, fakeCrd(shared.ModuleReleaseMetaKind), mrmApplier.crd)
}

func TestSyncCRDs_WhenKcpReadFails_AggregatesAndContinues(t *testing.T) {
	readErr := errors.New("kcp not reachable")
	kymaApplier := &skrCrdApplierStub{}
	mtApplier := &skrCrdApplierStub{}

	kcpRepo := &kcpCrdReaderStub{
		crdsByName: map[string]*apiextensionsv1.CustomResourceDefinition{
			crdName(shared.ModuleTemplateKind): fakeCrd(shared.ModuleTemplateKind),
		},
		errsByName: map[string]error{
			crdName(shared.KymaKind): readErr,
		},
	}

	svc := skrsync.NewService(kcpRepo, []skrsync.SkrCrdSyncEntry{
		{Kind: shared.KymaKind, Applier: kymaApplier},
		{Kind: shared.ModuleTemplateKind, Applier: mtApplier},
	}, nil, nil, "")

	err := svc.SyncCRDs(t.Context(), newKyma())

	require.Error(t, err)
	require.ErrorIs(t, err, skrsync.ErrCrdSync)
	require.ErrorIs(t, err, readErr)
	assert.False(t, kymaApplier.called)
	assert.True(t, mtApplier.called)
}

func TestSyncCRDs_WhenApplyFails_AggregatesError(t *testing.T) {
	applyErr := errors.New("apply failed")
	kymaApplier := &skrCrdApplierStub{err: applyErr}
	mtApplier := &skrCrdApplierStub{}

	kcpRepo := &kcpCrdReaderStub{
		crdsByName: map[string]*apiextensionsv1.CustomResourceDefinition{
			crdName(shared.KymaKind):           fakeCrd(shared.KymaKind),
			crdName(shared.ModuleTemplateKind): fakeCrd(shared.ModuleTemplateKind),
		},
	}

	svc := skrsync.NewService(kcpRepo, []skrsync.SkrCrdSyncEntry{
		{Kind: shared.KymaKind, Applier: kymaApplier},
		{Kind: shared.ModuleTemplateKind, Applier: mtApplier},
	}, nil, nil, "")

	err := svc.SyncCRDs(t.Context(), newKyma())

	require.Error(t, err)
	require.ErrorIs(t, err, skrsync.ErrCrdSync)
	require.ErrorIs(t, err, applyErr)
	assert.True(t, kymaApplier.called)
	assert.True(t, mtApplier.called)
}

// --- helpers & stubs ---

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

func fakeCrd(kind shared.Kind) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: apimetav1.ObjectMeta{Name: crdName(kind)},
	}
}

type kcpCrdReaderStub struct {
	crdsByName     map[string]*apiextensionsv1.CustomResourceDefinition
	errsByName     map[string]error
	requestedNames []string
}

func (s *kcpCrdReaderStub) Get(_ context.Context, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
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
