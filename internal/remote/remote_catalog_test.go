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

	_, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list ModuleReleaseMetas")
}

func Test_GetModuleReleaseMetasToSync_ReturnsNonBetaNonInternalMRM_ForNonBetaNonInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().build()

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mrms, 1)
	assert.Equal(t, "regular-module", mrms[0].Spec.ModuleName)
}

func Test_GetModuleReleaseMetasToSync_ReturnsBetaNonInternalMRM_ForBetaNonInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().build()

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mrms, 2)
	assert.Equal(t, "beta-module", mrms[0].Spec.ModuleName)
	assert.Equal(t, "regular-module", mrms[1].Spec.ModuleName)
}

func Test_GetModuleReleaseMetasToSync_ReturnsNonBetaInternalMRM_ForNonBetaInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withInternalEnabled().build()

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mrms, 2)
	assert.Equal(t, "internal-module", mrms[0].Spec.ModuleName)
	assert.Equal(t, "regular-module", mrms[1].Spec.ModuleName)
}

func Test_GetModuleReleaseMetasToSync_ReturnsBetaInternalMRM_ForBetaInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().withInternalEnabled().build()

	mrms, err := remoteCatalog.GetModuleReleaseMetasToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mrms, 4)
	assert.Equal(t, "beta-module", mrms[0].Spec.ModuleName)
	assert.Equal(t, "internal-beta-module", mrms[1].Spec.ModuleName)
	assert.Equal(t, "internal-module", mrms[2].Spec.ModuleName)
	assert.Equal(t, "regular-module", mrms[3].Spec.ModuleName)
}

func Test_GetModuleTemplatesToSync_ReturnsError_ForErrorClient(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(newErrorClient(), nil, "kyma-system")

	_, err := remoteCatalog.GetModuleTemplatesToSync(t.Context(), []v1beta2.ModuleReleaseMeta{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list ModuleTemplates")
}

func Test_GetModuleTemplatesToSync_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryNotSyncDisabled(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")

	mts, err := remoteCatalog.GetModuleTemplatesToSync(t.Context(), []v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("regular-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			withChannelVersion("sync-disabled", "3.0.0").
			withChannelVersion("mandatory", "4.0.0").
			build(),
	})

	require.NoError(t, err)
	require.Len(t, mts, 3)
	assert.Equal(t, "regular-module-1.0.0", mts[0].Name)
	assert.Equal(t, "regular-module-2.0.0", mts[1].Name)
}

func Test_FilterAllowedModuleTemplates_ReturnsMTsThatAreReferencedInMRMAndNotMandatoryNotSyncDisabled(t *testing.T) {
	mts := remote.FilterAllowedModuleTemplates(moduleTemplates().Items, []v1beta2.ModuleReleaseMeta{
		*newModuleReleaseMetaBuilder().
			withName("regular-module").
			withChannelVersion("regular", "1.0.0").
			withChannelVersion("fast", "2.0.0").
			withChannelVersion("sync-disabled", "3.0.0").
			withChannelVersion("mandatory", "4.0.0").
			build(),
	})

	require.Len(t, mts, 3)
	assert.Equal(t, "regular-module-1.0.0", mts[0].Name)
	assert.Equal(t, "regular-module-2.0.0", mts[1].Name)
}

func Test_GetOldModuleTemplatesToSync_ReturnsError_ForErrorClient(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(newErrorClient(), nil, "kyma-system")

	_, err := remoteCatalog.GetOldModuleTemplatesToSync(t.Context(), newKymaBuilder().build())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list ModuleTemplates")
}

func Test_GetOldModuleTemplatesToSync_ReturnsNonBetaNonInternalNonSyncDisabledNonMandatoryMTs_ForNonBetaNonInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().build()

	mts, err := remoteCatalog.GetOldModuleTemplatesToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mts, 2)
	assert.Equal(t, "old-module-regular", mts[0].Name)
}

func Test_GetOldModuleTemplatesToSync_ReturnsBetaNonInternalNonSyncDisabledNonMandatoryMTs_ForBetaNonInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().build()

	mts, err := remoteCatalog.GetOldModuleTemplatesToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mts, 3)
	assert.Equal(t, "old-beta-module-regular", mts[0].Name)
	assert.Equal(t, "old-module-regular", mts[1].Name)
}

func Test_GetOldModuleTemplatesToSync_ReturnsNonBetaInternalNonSyncDisabledNonMandatoryMTs_ForNonBetaInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withInternalEnabled().build()

	mts, err := remoteCatalog.GetOldModuleTemplatesToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mts, 3)
	assert.Equal(t, "old-internal-module-fast", mts[0].Name)
	assert.Equal(t, "old-module-regular", mts[1].Name)
}

func Test_GetOldModuleTemplatesToSync_ReturnsBetaInternalNonSyncDisabledNonMandatoryMTs_ForBetaInternalKyma(t *testing.T) {
	remoteCatalog := remote.NewRemoteCatalogFromKyma(fakeClient(), nil, "kyma-system")
	kyma := newKymaBuilder().withBetaEnabled().withInternalEnabled().build()

	mts, err := remoteCatalog.GetOldModuleTemplatesToSync(t.Context(), kyma)

	require.NoError(t, err)
	require.Len(t, mts, 5)
	assert.Equal(t, "old-beta-module-regular", mts[0].Name)
	assert.Equal(t, "old-internal-beta-module-fast", mts[1].Name)
	assert.Equal(t, "old-internal-module-fast", mts[2].Name)
	assert.Equal(t, "old-module-regular", mts[3].Name)
}

func Test_IsAllowedModuleReleaseMeta_ReturnsTrue_ForNonBetaNonInternalMRMAndNonBetaNonInternalKyma(t *testing.T) {
	mrm := newModuleReleaseMetaBuilder().build()
	kyma := newKymaBuilder().build()

	assert.True(t, remote.IsAllowedModuleReleaseMeta(*mrm, kyma))
}

func Test_IsAllowedModuleReleaseMeta(t *testing.T) {
	testCases := []struct {
		name           string
		moduleBeta     bool
		moduleInternal bool
		kymaBeta       bool
		kymaInternal   bool
		expected       bool
	}{
		{
			name:           "Given Module{Beta: false, Internal: false}; Kyma{Beta: false, Internal: false}; Expect Installation: true",
			moduleBeta:     false,
			moduleInternal: false,
			kymaBeta:       false,
			kymaInternal:   false,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: true, Internal: false}; Kyma{Beta: false, Internal: false}; Expect Installation:  false",
			moduleBeta:     true,
			moduleInternal: false,
			kymaBeta:       false,
			kymaInternal:   false,
			expected:       false,
		},
		{
			name:           "Given Module{Beta: false, Internal: true}; Kyma{Beta: false, Internal: false}; Expect Installation:  false",
			moduleBeta:     false,
			moduleInternal: true,
			kymaBeta:       false,
			kymaInternal:   false,
			expected:       false,
		},
		{
			name:           "Given Module{Beta: false, Internal: false}; Kyma{Beta: true, Internal: false}; Expect Installation:  true",
			moduleBeta:     false,
			moduleInternal: false,
			kymaBeta:       true,
			kymaInternal:   false,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: false, Internal: false}; Kyma{Beta: false, Internal: true}; Expect Installation:  true",
			moduleBeta:     false,
			moduleInternal: false,
			kymaBeta:       false,
			kymaInternal:   true,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: true, Internal: true}; Kyma{Beta: false, Internal: false}; Expect Installation:  false",
			moduleBeta:     true,
			moduleInternal: true,
			kymaBeta:       false,
			kymaInternal:   false,
			expected:       false,
		},
		{
			name:           "Given Module{Beta: true, Internal: false}; Kyma{Beta: true, Internal: false}; Expect Installation:  true",
			moduleBeta:     true,
			moduleInternal: false,
			kymaBeta:       true,
			kymaInternal:   false,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: true, Internal: false}; Kyma{Beta: false, Internal: true}; Expect Installation:  false",
			moduleBeta:     true,
			moduleInternal: false,
			kymaBeta:       false,
			kymaInternal:   true,
			expected:       false,
		},
		{
			name:           "Given Module{Beta: true, Internal: true}; Kyma{Beta: true, Internal: false}; Expect Installation:  false",
			moduleBeta:     true,
			moduleInternal: true,
			kymaBeta:       true,
			kymaInternal:   false,
			expected:       false,
		},
		{
			name:           "Given Module{Beta: true, Internal: true}; Kyma{Beta: false, Internal: true}; Expect Installation:  false",
			moduleBeta:     true,
			moduleInternal: true,
			kymaBeta:       false,
			kymaInternal:   true,
			expected:       false,
		},
		{
			name:           "Given Module{Beta: true, Internal: true}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			moduleBeta:     true,
			moduleInternal: true,
			kymaBeta:       true,
			kymaInternal:   true,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: false, Internal: true}; Kyma{Beta: true, Internal: false}; Expect Installation:  false",
			moduleBeta:     false,
			moduleInternal: true,
			kymaBeta:       true,
			kymaInternal:   false,
			expected:       false,
		},
		{
			name:           "Given Module{Beta: false, Internal: true}; Kyma{Beta: false, Internal: true}; Expect Installation:  true",
			moduleBeta:     false,
			moduleInternal: true,
			kymaBeta:       false,
			kymaInternal:   true,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: false, Internal: false}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			moduleBeta:     false,
			moduleInternal: false,
			kymaBeta:       true,
			kymaInternal:   true,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: false, Internal: true}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			moduleBeta:     false,
			moduleInternal: true,
			kymaBeta:       true,
			kymaInternal:   true,
			expected:       true,
		},
		{
			name:           "Given Module{Beta: true, Internal: false}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			moduleBeta:     true,
			moduleInternal: false,
			kymaBeta:       true,
			kymaInternal:   true,
			expected:       true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mrm := newModuleReleaseMetaBuilder().withBeta(testCase.moduleBeta).withInternal(testCase.moduleInternal).build()
			kyma := newKymaBuilder().withBeta(testCase.kymaBeta).withInternal(testCase.kymaInternal).build()

			result := remote.IsAllowedModuleReleaseMeta(*mrm, kyma)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

func moduleReleaseMetas() v1beta2.ModuleReleaseMetaList {
	mrm1 := newModuleReleaseMetaBuilder().
		withName("regular-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		withChannelVersion("sync-disabled", "3.0.0").
		withChannelVersion("mandatory", "4.0.0").
		build()
	mrm2 := newModuleReleaseMetaBuilder().
		withName("beta-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		withBetaEnabled().
		build()
	mrm3 := newModuleReleaseMetaBuilder().
		withName("internal-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		withInternalEnabled().
		build()
	mrm4 := newModuleReleaseMetaBuilder().
		withName("internal-beta-module").
		withChannelVersion("regular", "1.0.0").
		withChannelVersion("fast", "2.0.0").
		withBetaEnabled().
		withInternalEnabled().
		build()

	mrms := v1beta2.ModuleReleaseMetaList{
		Items: []v1beta2.ModuleReleaseMeta{
			*mrm1,
			*mrm2,
			*mrm3,
			*mrm4,
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

	// https://github.com/kyma-project/lifecycle-manager/issues/2096
	// Remove these after the migration to the new ModuleTemplate format is completed.
	mt7 := newModuleTemplateBuilder().
		withName("old-module-regular").
		withChannel("regular").
		build()
	mt8 := newModuleTemplateBuilder().
		withName("old-beta-module-regular").
		withChannel("regular").
		withBetaEnabled().
		build()
	mt9 := newModuleTemplateBuilder().
		withName("old-internal-module-fast").
		withChannel("fast").
		withInternalEnabled().
		build()
	mt10 := newModuleTemplateBuilder().
		withName("old-internal-beta-module-fast").
		withChannel("fast").
		withBetaEnabled().
		withInternalEnabled().
		build()
	mt11 := newModuleTemplateBuilder().
		withName("old-sync-disabled-module-experimental").
		withChannel("experimental").
		build()
	mt12 := newModuleTemplateBuilder().
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

func (b *moduleReleaseMetaBuilder) withBetaEnabled() *moduleReleaseMetaBuilder {
	b.moduleReleaseMeta.Spec.Beta = true
	return b
}

func (b *moduleReleaseMetaBuilder) withBeta(beta bool) *moduleReleaseMetaBuilder {
	b.moduleReleaseMeta.Spec.Beta = beta
	return b
}

func (b *moduleReleaseMetaBuilder) withInternalEnabled() *moduleReleaseMetaBuilder {
	b.moduleReleaseMeta.Spec.Internal = true
	return b
}

func (b *moduleReleaseMetaBuilder) withInternal(internal bool) *moduleReleaseMetaBuilder {
	b.moduleReleaseMeta.Spec.Internal = internal
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

func (b *kymaBuilder) withBeta(beta bool) *kymaBuilder {
	if beta {
		b.kyma.Labels[shared.BetaLabel] = shared.EnableLabelValue
	} else {
		b.kyma.Labels[shared.BetaLabel] = shared.DisableLabelValue
	}
	return b
}

func (b *kymaBuilder) withInternalEnabled() *kymaBuilder {
	b.kyma.Labels[shared.InternalLabel] = shared.EnableLabelValue
	return b
}

func (b *kymaBuilder) withInternal(internal bool) *kymaBuilder {
	if internal {
		b.kyma.Labels[shared.InternalLabel] = shared.EnableLabelValue
	} else {
		b.kyma.Labels[shared.InternalLabel] = shared.DisableLabelValue
	}
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
