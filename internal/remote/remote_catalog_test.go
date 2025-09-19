package remote_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

func Test_GetModuleReleaseMetasToSync_ReturnsError_ForErrorClient(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(newErrorClient(), nil, "kyma-system")
	kyma := newKymaBuilder().build()

	_, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list ModuleReleaseMetas")
}

func Test_GetModuleReleaseMetasToSync_ReturnsNonBetaNonInternalMRM_ForNonBetaNonInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma, mts)

	require.NoError(t, err)
	require.Len(t, mrms, 4)
	assert.Equal(t, "beta-module", mrms[0].Spec.ModuleName)
	assert.Len(t, mrms[0].Spec.Channels, 1)
	assert.Equal(t, "internal-beta-module", mrms[1].Spec.ModuleName)
	assert.Len(t, mrms[1].Spec.Channels, 1)
	assert.Equal(t, "internal-module", mrms[2].Spec.ModuleName)
	assert.Len(t, mrms[2].Spec.Channels, 1)
	assert.Equal(t, "regular-module", mrms[3].Spec.ModuleName)
	assert.Len(t, mrms[3].Spec.Channels, 3)
}

func Test_GetModuleReleaseMetasToSync_ReturnsBetaNonInternalMRM_ForBetaNonInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma, mts)

	require.NoError(t, err)
	require.Len(t, mrms, 5)
	assert.Equal(t, "beta-module", mrms[0].Spec.ModuleName)
	assert.Len(t, mrms[0].Spec.Channels, 2)
	assert.Equal(t, "beta-only-module", mrms[1].Spec.ModuleName)
	assert.Len(t, mrms[1].Spec.Channels, 1)
	assert.Equal(t, "internal-beta-module", mrms[2].Spec.ModuleName)
	assert.Len(t, mrms[2].Spec.Channels, 1)
	assert.Equal(t, "internal-module", mrms[3].Spec.ModuleName)
	assert.Len(t, mrms[3].Spec.Channels, 1)
}

func Test_GetModuleReleaseMetasToSync_ReturnsNonBetaInternalMRM_ForNonBetaInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withInternalEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma, mts)

	require.NoError(t, err)
	require.Len(t, mrms, 5)
	assert.Equal(t, "beta-module", mrms[0].Spec.ModuleName)
	assert.Len(t, mrms[0].Spec.Channels, 1)
	assert.Equal(t, "internal-beta-module", mrms[1].Spec.ModuleName)
	assert.Equal(t, "internal-module", mrms[2].Spec.ModuleName)
	assert.Len(t, mrms[2].Spec.Channels, 2)
	assert.Equal(t, "internal-only-module", mrms[3].Spec.ModuleName)
	assert.Len(t, mrms[3].Spec.Channels, 1)
}

func Test_GetModuleReleaseMetasToSync_ReturnsBetaInternalMRM_ForBetaInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().withInternalEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma, mts)

	require.NoError(t, err)
	require.Len(t, mrms, 6)
	assert.Equal(t, "beta-module", mrms[0].Spec.ModuleName)
	assert.Len(t, mrms[0].Spec.Channels, 2)
	assert.Equal(t, "beta-only-module", mrms[1].Spec.ModuleName)
	assert.Len(t, mrms[1].Spec.Channels, 1)
	assert.Equal(t, "internal-beta-module", mrms[2].Spec.ModuleName)
	assert.Len(t, mrms[2].Spec.Channels, 2)
	assert.Equal(t, "internal-module", mrms[3].Spec.ModuleName)
	assert.Len(t, mrms[3].Spec.Channels, 2)
	assert.Equal(t, "internal-only-module", mrms[4].Spec.ModuleName)
	assert.Len(t, mrms[4].Spec.Channels, 1)
	assert.Equal(t, "regular-module", mrms[5].Spec.ModuleName)
	assert.Len(t, mrms[5].Spec.Channels, 3)
}

func Test_GetModuleReleaseMetasToSync_SkipsMandatoryMRM_ForAnyKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma, mts)

	require.NoError(t, err)
	for _, mrm := range mrms {
		assert.NotEqual(t, "mandatory-module", mrm.Spec.ModuleName, "mandatory module should not be synced")
		assert.Nil(t, mrm.Spec.Mandatory, "synced MRM should not have mandatory field set")
	}
}

func Test_GetModuleTemplatesToSync_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryNotSyncDisabled(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	filteredMts, err := remoteCatalog.GetModuleTemplatesToSync([]v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("regular-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			withChannelVersion("sync-disabled", "3.0.0").
			withChannelVersion("mandatory", "4.0.0").
			build(),
	}, kyma, mts)

	require.NoError(t, err)
	require.Len(t, filteredMts, 3)
	assert.Equal(t, "regular-module-1.0.0", filteredMts[0].Name)
	assert.Equal(t, "regular-module-2.0.0", filteredMts[1].Name)
}

func Test_FilterAllowedModuleTemplates_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryNotSyncDisabled(t *testing.T) {
	kyma := newKymaBuilder().build()
	mts := remote.FilterAllowedModuleTemplates(moduleTemplates().Items, []v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("regular-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			withChannelVersion("sync-disabled", "3.0.0").
			withChannelVersion("mandatory", "4.0.0").
			build(),
	}, kyma)

	require.Len(t, mts, 3)
	assert.Equal(t, "regular-module-1.0.0", mts[0].Name)
	assert.Equal(t, "regular-module-2.0.0", mts[1].Name)
}

func Test_FilterAllowedModuleTemplates_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryInternal_ForInternalKyma(
	t *testing.T,
) {
	kyma := newKymaBuilder().withInternalEnabled().build()
	mts := remote.FilterAllowedModuleTemplates(moduleTemplates().Items, []v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("internal-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			build(),
	}, kyma)

	require.Len(t, mts, 2)
	assert.Equal(t, "internal-module-1.0.0", mts[0].Name)
	assert.Equal(t, "internal-module-2.0.0", mts[1].Name)
}

func Test_FilterAllowedModuleTemplates_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryBeta_ForBetaKyma(t *testing.T) {
	kyma := newKymaBuilder().withBetaEnabled().build()
	mts := remote.FilterAllowedModuleTemplates(moduleTemplates().Items, []v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("beta-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			build(),
	}, kyma)

	require.Len(t, mts, 2)
	assert.Equal(t, "beta-module-1.0.0", mts[0].Name)
	assert.Equal(t, "beta-module-2.0.0", mts[1].Name)
}

func Test_FilterAllowedModuleTemplates_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryInternalBeta_ForInternalBetaKyma(
	t *testing.T,
) {
	kyma := newKymaBuilder().withInternalEnabled().withBetaEnabled().build()
	mts := remote.FilterAllowedModuleTemplates(moduleTemplates().Items, []v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("internal-beta-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			build(),
	}, kyma)

	require.Len(t, mts, 2)
	assert.Equal(t, "internal-beta-module-1.0.0", mts[0].Name)
	assert.Equal(t, "internal-beta-module-2.0.0", mts[1].Name)
}

func Test_FilterAllowedModuleTemplates_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryInternal_ForNonInternalKyma(
	t *testing.T,
) {
	kyma := newKymaBuilder().build()
	mts := remote.FilterAllowedModuleTemplates(moduleTemplates().Items, []v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("internal-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			build(),
	}, kyma)

	require.Len(t, mts, 1)
}

func Test_GetOldModuleTemplatesToSync_ReturnsNonBetaNonInternalNonSyncDisabledNonMandatoryMTs_ForNonBetaNonInternalKyma(
	t *testing.T,
) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	filteredMts, err := remoteCatalog.GetOldModuleTemplatesToSync(kyma, mts)

	require.NoError(t, err)
	require.Len(t, filteredMts, 2)
	assert.Equal(t, "old-module-regular", filteredMts[0].Name)
}

func Test_GetOldModuleTemplatesToSync_ReturnsBetaNonInternalNonSyncDisabledNonMandatoryMTs_ForBetaNonInternalKyma(
	t *testing.T,
) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	filteredMts, err := remoteCatalog.GetOldModuleTemplatesToSync(kyma, mts)

	require.NoError(t, err)
	require.Len(t, filteredMts, 3)
	assert.Equal(t, "old-beta-module-regular", filteredMts[0].Name)
	assert.Equal(t, "old-module-regular", filteredMts[1].Name)
}

func Test_GetOldModuleTemplatesToSync_ReturnsNonBetaInternalNonSyncDisabledNonMandatoryMTs_ForNonBetaInternalKyma(
	t *testing.T,
) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withInternalEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	filteredMts, err := remoteCatalog.GetOldModuleTemplatesToSync(kyma, mts)

	require.NoError(t, err)
	require.Len(t, filteredMts, 3)
	assert.Equal(t, "old-internal-module-fast", filteredMts[0].Name)
	assert.Equal(t, "old-module-regular", filteredMts[1].Name)
}

func Test_GetOldModuleTemplatesToSync_ReturnsBetaInternalNonSyncDisabledNonMandatoryMTs_ForBetaInternalKyma(
	t *testing.T,
) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().withInternalEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	filteredMts, err := remoteCatalog.GetOldModuleTemplatesToSync(kyma, mts)

	require.NoError(t, err)
	require.Len(t, filteredMts, 5)
	assert.Equal(t, "old-beta-module-regular", filteredMts[0].Name)
	assert.Equal(t, "old-internal-beta-module-fast", filteredMts[1].Name)
	assert.Equal(t, "old-internal-module-fast", filteredMts[2].Name)
	assert.Equal(t, "old-module-regular", filteredMts[3].Name)
}

func Test_IsAllowedModuleVersion_ForNonBetaInternalKyma_NoBetaInternalModule(t *testing.T) {
	kyma := newKymaBuilder().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	isAllowed := remote.IsAllowedModuleVersion(kyma, mts, "regular-module", "1.0.0")
	require.True(t, isAllowed)
}

func Test_IsAllowedModuleVersion_ForNonBetaInternalKyma_BetaModule(t *testing.T) {
	kyma := newKymaBuilder().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	isAllowed := remote.IsAllowedModuleVersion(kyma, mts, "beta-module", "1.0.0")
	require.False(t, isAllowed)
}

func Test_IsAllowedModuleVersion_ForNonBetaInternalKyma_InternalModule(t *testing.T) {
	kyma := newKymaBuilder().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	isAllowed := remote.IsAllowedModuleVersion(kyma, mts, "internal-module", "1.0.0")
	require.False(t, isAllowed)
}

func Test_IsAllowedModuleVersion_ForBetaKyma_InternalModule(t *testing.T) {
	kyma := newKymaBuilder().withBetaEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	isAllowed := remote.IsAllowedModuleVersion(kyma, mts, "internal-module", "1.0.0")
	require.False(t, isAllowed)
}

func Test_IsAllowedModuleVersion_ForBetaKyma_BetaModule(t *testing.T) {
	kyma := newKymaBuilder().withBetaEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	isAllowed := remote.IsAllowedModuleVersion(kyma, mts, "beta-module", "1.0.0")
	require.True(t, isAllowed)
}

func Test_IsAllowedModuleVersion_ForInternalKyma_BetaModule(t *testing.T) {
	kyma := newKymaBuilder().withInternalEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	isAllowed := remote.IsAllowedModuleVersion(kyma, mts, "beta-module", "1.0.0")
	require.False(t, isAllowed)
}

func Test_IsAllowedModuleVersion_ForInternalKyma_InternalModule(t *testing.T) {
	kyma := newKymaBuilder().withInternalEnabled().build()
	mts := &v1beta2.ModuleTemplateList{}
	err := fakeClient().List(t.Context(), mts)
	require.NoError(t, err)

	isAllowed := remote.IsAllowedModuleVersion(kyma, mts, "internal-module", "1.0.0")
	require.True(t, isAllowed)
}

func moduleReleaseMetas() v1beta2.ModuleReleaseMetaList {
	mrm1 := newModuleReleaseMetaBuilder().
		withName("regular-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		withChannelVersion("sync-disabled", "3.0.0").
		build()
	mrm2 := newModuleReleaseMetaBuilder().
		withName("beta-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		build()
	mrm3 := newModuleReleaseMetaBuilder().
		withName("internal-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		build()
	mrm4 := newModuleReleaseMetaBuilder().
		withName("internal-beta-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		build()
	mrm5 := newModuleReleaseMetaBuilder().
		withName("beta-only-module").
		withChannelVersion("regular", "1.0.0").
		build()
	mrm6 := newModuleReleaseMetaBuilder().
		withName("internal-only-module").
		withChannelVersion("regular", "1.0.0").
		build()
	mandatoryMRM := newModuleReleaseMetaBuilder().
		withName("mandatory-module").
		withMandatory("1.0.0").
		build()

	mrms := v1beta2.ModuleReleaseMetaList{
		Items: []v1beta2.ModuleReleaseMeta{
			*mrm1,
			*mrm2,
			*mrm3,
			*mrm4,
			*mrm5,
			*mrm6,
			*mandatoryMRM,
		},
	}

	return mrms
}

func moduleTemplates() v1beta2.ModuleTemplateList {
	mt1 := newModuleTemplateBuilder().
		withName("regular-module-1.0.0").
		withModuleName("regular-module").
		withVersion("1.0.0").
		build()
	mt2 := newModuleTemplateBuilder().
		withName("regular-module-2.0.0").
		withModuleName("regular-module").
		withVersion("2.0.0").
		build()
	mt3 := newModuleTemplateBuilder().
		withName("regular-module-3.0.0").
		withModuleName("regular-module").
		withVersion("3.0.0").
		build()
	mt4 := newModuleTemplateBuilder().
		withName("regular-module-4.0.0").
		withModuleName("regular-module").
		withVersion("4.0.0").
		withMandatoryEnabled().
		build()
	mt5 := newModuleTemplateBuilder().
		withName("not-referenced-module-1.0.0").
		withModuleName("not-referenced-module").
		withVersion("1.0.0").
		build()
	mt6 := newModuleTemplateBuilder().
		withName("not-referenced-module-2.0.0").
		withModuleName("not-referenced-module").
		withVersion("2.0.0").
		build()
	mt7 := newModuleTemplateBuilder().
		withName("internal-module-1.0.0").
		withModuleName("internal-module").
		withVersion("1.0.0").
		withInternalEnabled().
		build()
	mt8 := newModuleTemplateBuilder().
		withName("beta-module-1.0.0").
		withModuleName("beta-module").
		withVersion("1.0.0").
		withBetaEnabled().
		build()
	mt9 := newModuleTemplateBuilder().
		withName("internal-module-2.0.0").
		withModuleName("internal-module").
		withVersion("2.0.0").
		build()
	mt10 := newModuleTemplateBuilder().
		withName("beta-module-2.0.0").
		withModuleName("beta-module").
		withVersion("2.0.0").
		build()
	mt11 := newModuleTemplateBuilder().
		withName("internal-beta-module-1.0.0").
		withModuleName("internal-beta-module").
		withVersion("1.0.0").
		withInternalEnabled().
		withBetaEnabled().
		build()
	mt12 := newModuleTemplateBuilder().
		withName("internal-beta-module-2.0.0").
		withModuleName("internal-beta-module").
		withVersion("2.0.0").
		build()
	mt13 := newModuleTemplateBuilder().
		withName("internal-only-module-1.0.0").
		withModuleName("internal-only-module").
		withVersion("1.0.0").
		withInternalEnabled().
		build()
	mt14 := newModuleTemplateBuilder().
		withName("beta-only-module-1.0.0").
		withModuleName("beta-only-module").
		withVersion("1.0.0").
		withBetaEnabled().
		build()

	// https://github.com/kyma-project/lifecycle-manager/issues/2096
	// Remove these after the migration to the new ModuleTemplate format is completed.
	mt15 := newModuleTemplateBuilder().
		withName("old-module-regular").
		withChannel("regular").
		build()
	mt16 := newModuleTemplateBuilder().
		withName("old-beta-module-regular").
		withChannel("regular").
		withBetaEnabled().
		build()
	mt17 := newModuleTemplateBuilder().
		withName("old-internal-module-fast").
		withChannel("fast").
		withInternalEnabled().
		build()
	mt18 := newModuleTemplateBuilder().
		withName("old-internal-beta-module-fast").
		withChannel("fast").
		withBetaEnabled().
		withInternalEnabled().
		build()
	mt19 := newModuleTemplateBuilder().
		withName("old-sync-disabled-module-experimental").
		withChannel("experimental").
		build()
	mt20 := newModuleTemplateBuilder().
		withName("old-mandatory-module").
		withChannel("regular").
		withMandatoryEnabled().
		build()

	mts := v1beta2.ModuleTemplateList{
		Items: []v1beta2.ModuleTemplate{
			*mt1,
			*mt2,
			*mt3,
			*mt4,
			*mt5,
			*mt6,
			*mt7,
			*mt8,
			*mt9,
			*mt10,
			*mt11,
			*mt12,
			*mt13,
			*mt14,
			*mt15,
			*mt16,
			*mt17,
			*mt18,
			*mt19,
			*mt20,
		},
	}

	return mts
}

func fakeClient() client.Client {
	mrms := moduleReleaseMetas()
	mts := moduleTemplates()

	scheme := machineryruntime.NewScheme()
	machineryutilruntime.Must(api.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithLists(&mrms, &mts).Build()
}

type moduleReleaseMetaBuilder struct {
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta
}

func newModuleReleaseMetaBuilder() *moduleReleaseMetaBuilder {
	return &moduleReleaseMetaBuilder{
		moduleReleaseMeta: &v1beta2.ModuleReleaseMeta{
			Spec: v1beta2.ModuleReleaseMetaSpec{
				Channels: []v1beta2.ChannelVersionAssignment{},
			},
		},
	}
}

func (b *moduleReleaseMetaBuilder) build() *v1beta2.ModuleReleaseMeta {
	return b.moduleReleaseMeta
}

func (b *moduleReleaseMetaBuilder) withName(module string) *moduleReleaseMetaBuilder {
	b.moduleReleaseMeta.Name = module
	b.moduleReleaseMeta.Spec.ModuleName = module
	return b
}

func (b *moduleReleaseMetaBuilder) withChannelVersion(channel, version string) *moduleReleaseMetaBuilder {
	b.moduleReleaseMeta.Spec.Channels = append(
		b.moduleReleaseMeta.Spec.Channels,
		v1beta2.ChannelVersionAssignment{Channel: channel, Version: version},
	)
	return b
}

func (b *moduleReleaseMetaBuilder) withMandatory(version string) *moduleReleaseMetaBuilder {
	b.moduleReleaseMeta.Spec.Mandatory = &v1beta2.Mandatory{
		Version: version,
	}
	// Clear channels as mandatory modules cannot have channels (per validation rule)
	b.moduleReleaseMeta.Spec.Channels = nil
	return b
}

type moduleTemplateBuilder struct {
	moduleTemplate *v1beta2.ModuleTemplate
}

func newModuleTemplateBuilder() *moduleTemplateBuilder {
	return &moduleTemplateBuilder{
		moduleTemplate: &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Labels: map[string]string{},
			},
		},
	}
}

func (b *moduleTemplateBuilder) build() *v1beta2.ModuleTemplate {
	return b.moduleTemplate
}

func (b *moduleTemplateBuilder) withName(name string) *moduleTemplateBuilder {
	b.moduleTemplate.Name = name
	return b
}

func (b *moduleTemplateBuilder) withVersion(version string) *moduleTemplateBuilder {
	b.moduleTemplate.Spec.Version = version
	return b
}

func (b *moduleTemplateBuilder) withModuleName(module string) *moduleTemplateBuilder {
	b.moduleTemplate.Spec.ModuleName = module
	return b
}

func (b *moduleTemplateBuilder) withChannel(channel string) *moduleTemplateBuilder {
	b.moduleTemplate.Spec.Channel = channel
	return b
}

func (b *moduleTemplateBuilder) withMandatoryEnabled() *moduleTemplateBuilder {
	b.moduleTemplate.Spec.Mandatory = true
	b.moduleTemplate.Labels[shared.IsMandatoryModule] = shared.EnableLabelValue
	return b
}

func (b *moduleTemplateBuilder) withBetaEnabled() *moduleTemplateBuilder {
	b.moduleTemplate.Labels[shared.BetaLabel] = shared.EnableLabelValue
	return b
}

func (b *moduleTemplateBuilder) withInternalEnabled() *moduleTemplateBuilder {
	b.moduleTemplate.Labels[shared.InternalLabel] = shared.EnableLabelValue
	return b
}

type kymaBuilder struct {
	kyma *v1beta2.Kyma
}

func newKymaBuilder() *kymaBuilder {
	return &kymaBuilder{
		kyma: &v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{
				Labels: map[string]string{},
			},
		},
	}
}

func (b *kymaBuilder) build() *v1beta2.Kyma {
	return b.kyma
}

func (b *kymaBuilder) withBetaEnabled() *kymaBuilder {
	b.kyma.Labels[shared.BetaLabel] = shared.EnableLabelValue
	return b
}

func (b *kymaBuilder) withInternalEnabled() *kymaBuilder {
	b.kyma.Labels[shared.InternalLabel] = shared.EnableLabelValue
	return b
}

type errorClient struct {
	client.Client
}

func newErrorClient() errorClient {
	return errorClient{
		Client: fake.NewClientBuilder().WithScheme(machineryruntime.NewScheme()).Build(),
	}
}

func (c errorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return assert.AnError
}
