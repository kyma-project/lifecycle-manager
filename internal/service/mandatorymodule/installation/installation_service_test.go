package installation_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	installerrors "github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/installation"
	installservice "github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/installation"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestHandleInstallation_KymaBeingDeleted_ReturnsErrKymaBeingDeleted(t *testing.T) {
	svc := installservice.NewService(&mrmRepoStub{}, &mtRepoStub{},
		&moduleParserStub{}, &manifestCreatorStub{}, &metricsStub{})
	kyma := builder.NewKymaBuilder().WithDeletionTimestamp().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.ErrorIs(t, err, installerrors.ErrKymaBeingDeleted)
}

func TestHandleInstallation_SkipReconciliation_ReturnsErrSkipReconcileKyma(t *testing.T) {
	svc := installservice.NewService(&mrmRepoStub{}, &mtRepoStub{}, &moduleParserStub{},
		&manifestCreatorStub{}, &metricsStub{})
	kyma := builder.NewKymaBuilder().
		WithLabel(shared.SkipReconcileLabel, shared.EnableLabelValue).
		Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.ErrorIs(t, err, installerrors.ErrSkipReconcileKyma)
}

func TestHandleInstallation_ListMandatoryFails_ReturnsError(t *testing.T) {
	mrmRepo := &mrmRepoStub{listErr: assert.AnError}
	svc := installservice.NewService(mrmRepo, &mtRepoStub{}, &moduleParserStub{},
		&manifestCreatorStub{}, &metricsStub{})
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "list mandatory modules failed")
}

func TestHandleInstallation_NoMandatoryModules_Success(t *testing.T) {
	mrmRepo := &mrmRepoStub{mrms: []v1beta2.ModuleReleaseMeta{}}
	metrics := &metricsStub{}
	parser := &moduleParserStub{}
	creator := &manifestCreatorStub{}
	svc := installservice.NewService(mrmRepo, &mtRepoStub{}, parser, creator, metrics)
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.NoError(t, err)
	require.Equal(t, 0, metrics.recordedCount)
	require.True(t, parser.called)
	require.True(t, creator.called)
}

func TestHandleInstallation_MrmBeingDeleted_SkippedInTemplates(t *testing.T) {
	mrm := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithMandatory("1.0.0").
		WithOcmComponentName("github.com/kyma-project/test-module").
		Build()
	mrm.DeletionTimestamp = &apimetav1.Time{Time: time.Now()}

	mrmRepo := &mrmRepoStub{mrms: []v1beta2.ModuleReleaseMeta{*mrm}}
	metrics := &metricsStub{}
	parser := &moduleParserStub{}
	creator := &manifestCreatorStub{}
	svc := installservice.NewService(mrmRepo, &mtRepoStub{}, parser, creator, metrics)
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.NoError(t, err)
	require.Equal(t, 1, metrics.recordedCount)
	require.True(t, parser.called)
	require.Empty(t, parser.receivedTemplates)
}

func TestHandleInstallation_GetModuleTemplateFails_ReturnsError(t *testing.T) {
	mrm := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithMandatory("1.0.0").
		WithOcmComponentName("github.com/kyma-project/test-module").
		Build()
	mrmRepo := &mrmRepoStub{mrms: []v1beta2.ModuleReleaseMeta{*mrm}}
	mtRepo := &mtRepoStub{getErr: assert.AnError}
	svc := installservice.NewService(mrmRepo, mtRepo, &moduleParserStub{}, &manifestCreatorStub{}, &metricsStub{})
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "get ModuleTemplate for mandatory module test-module failed")
}

func TestHandleInstallation_SingleMandatoryModule_Success(t *testing.T) {
	mrm := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithMandatory("1.0.0").
		WithOcmComponentName("github.com/kyma-project/test-module").
		Build()
	mt := builder.NewModuleTemplateBuilder().
		WithModuleName("test-module").
		WithVersion("1.0.0").
		Build()

	mrmRepo := &mrmRepoStub{mrms: []v1beta2.ModuleReleaseMeta{*mrm}}
	mtRepo := &mtRepoStub{
		templates: map[mtRepoKey]*v1beta2.ModuleTemplate{
			{name: "test-module", version: "1.0.0"}: mt,
		},
	}
	metrics := &metricsStub{}
	parser := &moduleParserStub{}
	creator := &manifestCreatorStub{}
	svc := installservice.NewService(mrmRepo, mtRepo, parser, creator, metrics)
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.NoError(t, err)
	require.Equal(t, 1, metrics.recordedCount)
	require.True(t, parser.called)
	require.Contains(t, parser.receivedTemplates, "test-module")
	templateInfo := parser.receivedTemplates["test-module"]
	require.NotNil(t, templateInfo)
	require.NoError(t, templateInfo.Err)
	require.True(t, templateInfo.Spec.Mandatory)
	require.NotNil(t, templateInfo.ComponentId)
	require.Equal(t, "github.com/kyma-project/test-module", templateInfo.ComponentId.Name())
	require.Equal(t, "1.0.0", templateInfo.ComponentId.Version())
	require.True(t, creator.called)
}

func TestHandleInstallation_EmptyOcmComponentName_TemplateInfoContainsError(t *testing.T) {
	mrm := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithMandatory("1.0.0").
		Build()
	// OcmComponentName is empty by default
	mt := builder.NewModuleTemplateBuilder().
		WithModuleName("test-module").
		WithVersion("1.0.0").
		Build()

	mrmRepo := &mrmRepoStub{mrms: []v1beta2.ModuleReleaseMeta{*mrm}}
	mtRepo := &mtRepoStub{
		templates: map[mtRepoKey]*v1beta2.ModuleTemplate{
			{name: "test-module", version: "1.0.0"}: mt,
		},
	}
	parser := &moduleParserStub{}
	creator := &manifestCreatorStub{}
	svc := installservice.NewService(mrmRepo, mtRepo, parser, creator, &metricsStub{})
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.NoError(t, err)
	require.Contains(t, parser.receivedTemplates, "test-module")
	templateInfo := parser.receivedTemplates["test-module"]
	require.Error(t, templateInfo.Err)
	require.ErrorContains(t, templateInfo.Err, "failed creating OCM identity")
	require.Nil(t, templateInfo.ComponentId)
	require.True(t, templateInfo.Spec.Mandatory)
}

func TestHandleInstallation_MultipleMandatoryModules_AllProcessed(t *testing.T) {
	mrm1 := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("module-aaa").
		WithMandatory("1.0.0").
		WithOcmComponentName("github.com/kyma-project/module-aaa").
		Build()
	mrm2 := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("module-bbb").
		WithMandatory("2.0.0").
		WithOcmComponentName("github.com/kyma-project/module-bbb").
		Build()
	mt1 := builder.NewModuleTemplateBuilder().
		WithModuleName("module-aaa").
		WithVersion("1.0.0").
		Build()
	mt2 := builder.NewModuleTemplateBuilder().
		WithModuleName("module-bbb").
		WithVersion("2.0.0").
		Build()

	mrmRepo := &mrmRepoStub{mrms: []v1beta2.ModuleReleaseMeta{*mrm1, *mrm2}}
	mtRepo := &mtRepoStub{
		templates: map[mtRepoKey]*v1beta2.ModuleTemplate{
			{name: "module-aaa", version: "1.0.0"}: mt1,
			{name: "module-bbb", version: "2.0.0"}: mt2,
		},
	}
	metrics := &metricsStub{}
	parser := &moduleParserStub{}
	creator := &manifestCreatorStub{}
	svc := installservice.NewService(mrmRepo, mtRepo, parser, creator, metrics)
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.NoError(t, err)
	require.Equal(t, 2, metrics.recordedCount)
	require.Len(t, parser.receivedTemplates, 2)
	require.Contains(t, parser.receivedTemplates, "module-aaa")
	require.Contains(t, parser.receivedTemplates, "module-bbb")
}

func TestHandleInstallation_ReconcileManifestsFails_ReturnsError(t *testing.T) {
	mrmRepo := &mrmRepoStub{mrms: []v1beta2.ModuleReleaseMeta{}}
	creator := &manifestCreatorStub{err: assert.AnError}
	svc := installservice.NewService(mrmRepo, &mtRepoStub{}, &moduleParserStub{}, creator, &metricsStub{})
	kyma := builder.NewKymaBuilder().Build()

	err := svc.HandleInstallation(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "reconcile manifests for mandatory modules failed")
}

// Test stubs

type mrmRepoStub struct {
	mrms    []v1beta2.ModuleReleaseMeta
	listErr error
}

func (s *mrmRepoStub) ListMandatory(_ context.Context) ([]v1beta2.ModuleReleaseMeta, error) {
	return s.mrms, s.listErr
}

type mtRepoKey struct {
	name    string
	version string
}

type mtRepoStub struct {
	templates map[mtRepoKey]*v1beta2.ModuleTemplate
	getErr    error
}

func (s *mtRepoStub) GetSpecificVersionForModule(_ context.Context,
	moduleName, version string,
) (*v1beta2.ModuleTemplate, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.templates != nil {
		if mt, ok := s.templates[mtRepoKey{name: moduleName, version: version}]; ok {
			return mt, nil
		}
	}
	return nil, assert.AnError
}

type moduleParserStub struct {
	called            bool
	receivedTemplates templatelookup.ModuleTemplatesByModuleName
}

func (s *moduleParserStub) GenerateMandatoryModulesFromTemplates(
	_ context.Context,
	_ *v1beta2.Kyma,
	templates templatelookup.ModuleTemplatesByModuleName,
) modulecommon.Modules {
	s.called = true
	s.receivedTemplates = templates
	return nil
}

type manifestCreatorStub struct {
	called bool
	err    error
}

func (s *manifestCreatorStub) ReconcileManifests(
	_ context.Context,
	_ *v1beta2.Kyma,
	_ modulecommon.Modules,
) error {
	s.called = true
	return s.err
}

type metricsStub struct {
	recordedCount int
}

func (s *metricsStub) RecordMandatoryModulesCount(count int) {
	s.recordedCount = count
}
