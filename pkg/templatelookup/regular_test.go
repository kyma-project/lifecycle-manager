package templatelookup_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"ocm.software/ocm/api/ocm/compdesc"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	descriptorcache "github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/service/restrictedmodule"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/service/componentdescriptor"
)

const (
	version1 = "1.0.1"
	version2 = "2.2.0"
	version3 = "3.0.3"

	versionUpgradeErr = "as a higher version (" + version3 + ") of the module was previously installed"
)

type FakeModuleTemplateReader struct {
	templateList          v1beta2.ModuleTemplateList
	moduleReleaseMetaList v1beta2.ModuleReleaseMetaList
}

func NewFakeModuleTemplateReader(templateList v1beta2.ModuleTemplateList,
	moduleReleaseMetaList v1beta2.ModuleReleaseMetaList,
) *FakeModuleTemplateReader {
	return &FakeModuleTemplateReader{
		templateList:          templateList,
		moduleReleaseMetaList: moduleReleaseMetaList,
	}
}

func (f *FakeModuleTemplateReader) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	castedList, ok := list.(*v1beta2.ModuleTemplateList)
	if !ok {
		return errors.New("list is not of type *v1beta2.ModuleTemplateList")
	}
	castedList.Items = append(castedList.Items, f.templateList.Items...)
	return nil
}

func (f *FakeModuleTemplateReader) Get(_ context.Context, objKey client.ObjectKey, obj client.Object,
	_ ...client.GetOption,
) error {
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{}, objKey.Name)
	if castedObj, ok := obj.(*v1beta2.ModuleReleaseMeta); ok {
		for _, mrm := range f.moduleReleaseMetaList.Items {
			if mrm.Name == objKey.Name {
				*castedObj = mrm
				return nil
			}
		}

		return notFoundErr
	} else if castedObj, ok := obj.(*v1beta2.ModuleTemplate); ok {
		for _, template := range f.templateList.Items {
			if template.Name == objKey.Name {
				*castedObj = template
				return nil
			}
		}
		return notFoundErr
	}

	return nil
}

func TestValidateTemplateMode_ForOldModuleTemplates(t *testing.T) {
	tests := []struct {
		name     string
		template templatelookup.ModuleTemplateInfo
		kyma     *v1beta2.Kyma
		wantErr  error
	}{
		{
			name: "When TemplateInfo contains Error, Then the output is same as input",
			template: templatelookup.ModuleTemplateInfo{
				Err: templatelookup.ErrTemplateNotAllowed,
			},
			wantErr: templatelookup.ErrTemplateNotAllowed,
		},
		{
			name: "When ModuleTemplate is internal but Kyma is not, Then result contains error",
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().
					WithLabel(shared.InternalLabel, "true").Build(),
			},
			kyma: builder.NewKymaBuilder().
				WithLabel(shared.InternalLabel, "false").
				Build(),
			wantErr: templatelookup.ErrTemplateNotAllowed,
		},
		{
			name: "When ModuleTemplate is beta but Kyma is not, Then result contains error",
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().
					WithLabel(shared.BetaLabel, "true").Build(),
			},
			kyma: builder.NewKymaBuilder().
				WithLabel(shared.BetaLabel, "false").
				Build(),
			wantErr: templatelookup.ErrTemplateNotAllowed,
		},
		{
			name: "When ModuleTemplate is mandatory, Then result contains error",
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().
					WithMandatory(true).Build(),
			},
			kyma:    builder.NewKymaBuilder().Build(),
			wantErr: common.ErrNoTemplatesInListResult,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			lookup := templatelookup.NewTemplateLookup(nil, nil, nil, nil)
			if got := lookup.ValidateTemplateMode(testCase.template, testCase.kyma, nil); !errors.Is(
				got.Err, testCase.wantErr) {
				t.Errorf("ValidateTemplateMode() = %v, want %v", got, testCase.wantErr)
			}
		})
	}
}

func TestValidateTemplateMode_RestrictedModules(t *testing.T) {
	const restrictedModuleName = "restricted-module"

	tests := []struct {
		name              string
		restrictedModules []string
		mrm               *v1beta2.ModuleReleaseMeta
		kyma              *v1beta2.Kyma
		template          templatelookup.ModuleTemplateInfo
		wantErr           error
	}{
		{
			name:              "When module in restricted list and nil selector, Then not allowed",
			restrictedModules: []string{restrictedModuleName},
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithKymaSelector(nil).
				Build(),
			kyma: builder.NewKymaBuilder().
				WithLabel("kyma-project.io/global-account-id", "account-1").
				Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: templatelookup.ErrTemplateNotAllowed,
		},
		{
			name:              "When module in restricted list and empty selector, Then not allowed",
			restrictedModules: []string{restrictedModuleName},
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithKymaSelector(&apimetav1.LabelSelector{}).
				Build(),
			kyma: builder.NewKymaBuilder().
				WithLabel("kyma-project.io/global-account-id", "account-1").
				Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: templatelookup.ErrTemplateNotAllowed,
		},
		{
			name:              "When module in restricted list and matching selector, Then allowed",
			restrictedModules: []string{restrictedModuleName},
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithGlobalAccountKymaSelector("account-1", "account-2").
				Build(),
			kyma: builder.NewKymaBuilder().
				WithLabel("kyma-project.io/global-account-id", "account-1").
				Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: nil,
		},
		{
			name:              "When module in restricted list and non-matching selector, Then not allowed",
			restrictedModules: []string{restrictedModuleName},
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithGlobalAccountKymaSelector("account-1", "account-2").
				Build(),
			kyma: builder.NewKymaBuilder().
				WithLabel("kyma-project.io/global-account-id", "account-3").
				Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: templatelookup.ErrTemplateNotAllowed,
		},
		{
			name:              "When module NOT in restricted list, Then allowed regardless of selector",
			restrictedModules: []string{"other-module"},
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithKymaSelector(nil).
				Build(),
			kyma: builder.NewKymaBuilder().Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: nil,
		},
		{
			name:              "When restricted modules list is empty, Then allowed",
			restrictedModules: nil,
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithKymaSelector(nil).
				Build(),
			kyma: builder.NewKymaBuilder().Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: nil,
		},
		{
			name:              "When module in restricted list and selector parse error, Then not allowed",
			restrictedModules: []string{restrictedModuleName},
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithKymaSelector(&apimetav1.LabelSelector{
					MatchExpressions: []apimetav1.LabelSelectorRequirement{
						{
							Key:      "kyma-project.io/global-account-id",
							Operator: "InvalidOperator",
							Values:   []string{"account-1"},
						},
					},
				}).
				Build(),
			kyma: builder.NewKymaBuilder().
				WithLabel("kyma-project.io/global-account-id", "account-1").
				Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: restrictedmodule.ErrSelectorParse,
		},
		{
			name:              "When module in restricted list and matching matchLabels selector, Then allowed",
			restrictedModules: []string{restrictedModuleName},
			mrm: builder.NewModuleReleaseMetaBuilder().
				WithModuleName(restrictedModuleName).
				WithKymaSelector(&apimetav1.LabelSelector{
					MatchLabels: map[string]string{
						"kyma-project.io/global-account-id": "account-1",
					},
				}).
				Build(),
			kyma: builder.NewKymaBuilder().
				WithLabel("kyma-project.io/global-account-id", "account-1").
				Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: nil,
		},
		{
			name:              "When MRM is nil, Then allowed",
			restrictedModules: []string{restrictedModuleName},
			mrm:               nil,
			kyma:              builder.NewKymaBuilder().Build(),
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().Build(),
			},
			wantErr: nil,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			lookup := templatelookup.NewTemplateLookup(nil, nil, nil, testCase.restrictedModules)
			got := lookup.ValidateTemplateMode(testCase.template, testCase.kyma, testCase.mrm)
			if testCase.wantErr != nil {
				require.ErrorIs(t, got.Err, testCase.wantErr)
			} else {
				require.NoError(t, got.Err)
			}
		})
	}
}

func Test_GetRegularTemplates_WhenInvalidModuleProvided(t *testing.T) {
	tests := []struct {
		name       string
		KymaSpec   v1beta2.KymaSpec
		KymaStatus v1beta2.KymaStatus
		wantErr    error
	}{
		{
			name: "When Module in Spec contains both Channel and Version, Then result contains error",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Channel: "regular", Version: "v1.0"},
				},
			},
			wantErr: templatelookup.ErrInvalidModuleInSpec,
		},
		{
			name: "When Template not exists in Status, Then result contains error",
			KymaStatus: v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{
					{
						Name:     "Module1",
						Channel:  "regular",
						Version:  "v1.0",
						Template: nil,
					},
				},
			},
			wantErr: templatelookup.ErrInvalidModuleInStatus,
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(tt.name, func(t *testing.T) {
			lookup := templatelookup.NewTemplateLookup(
				nil,
				nil, // not used in tests
				moduletemplateinfolookup.NewLookup(nil),
				nil,
			)
			kyma := &v1beta2.Kyma{
				Spec:   test.KymaSpec,
				Status: test.KymaStatus,
			}
			got := lookup.GetRegularTemplates(t.Context(), kyma)
			for _, err := range got {
				if !errors.Is(err.Err, test.wantErr) {
					t.Errorf("GetRegularTemplates() = %v, want %v", got, test.wantErr)
				}
			}
		})
	}
}

func TestTemplateLookup_GetRegularTemplates_WhenSwitchModuleChannel(t *testing.T) {
	testModule := testutils.NewTestModule("module1", "new_channel")

	fakeService := &componentdescriptor.FakeService{}
	descriptorProvider := provider.NewCachedDescriptorProvider(
		fakeService,
		descriptorcache.NewDescriptorCache(),
	)
	err := registerEmptyComponentDescriptor(fakeService, testutils.FullOCMName(testModule.Name), version1)
	require.NoError(t, err)
	err = registerEmptyComponentDescriptor(fakeService, testutils.FullOCMName(testModule.Name), version2)
	require.NoError(t, err)

	tests := []struct {
		name                       string
		kyma                       *v1beta2.Kyma
		availableModuleTemplate    v1beta2.ModuleTemplateList
		availableModuleReleaseMeta v1beta2.ModuleReleaseMetaList
		want                       templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When upgrade version during channel switch, " +
				"then result contains no error, with ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    testModule.Name,
					Channel: v1beta2.DefaultChannel,
					Version: version1,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, "",
				version2),
			availableModuleReleaseMeta: generateModuleReleaseMetaList(testModule.Name,
				[]v1beta2.ChannelVersionAssignment{
					{Channel: testModule.Channel, Version: version2},
					{Channel: v1beta2.DefaultChannel, Version: version1},
				}),
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            nil,
				},
			},
		},
		{
			name: "When downgrade version during channel switch, " +
				"then result contains error, with ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    testModule.Name,
					Version: version2,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, "",
				version1),
			availableModuleReleaseMeta: generateModuleReleaseMetaList(testModule.Name,
				[]v1beta2.ChannelVersionAssignment{
					{Channel: testModule.Channel, Version: version1},
				}),
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            templatelookup.ErrTemplateUpdateNotAllowed,
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			reader := NewFakeModuleTemplateReader(
				testCase.availableModuleTemplate,
				testCase.availableModuleReleaseMeta,
			)
			lookup := templatelookup.NewTemplateLookup(
				reader,
				descriptorProvider,
				moduletemplateinfolookup.NewLookup(reader),
				nil,
			)
			got := lookup.GetRegularTemplates(t.Context(), testCase.kyma)
			expected := len(testCase.want)
			assert.Len(t, got, expected)
			for key, module := range got {
				wantModule, ok := testCase.want[key]
				assert.True(t, ok)
				assert.Equal(t, wantModule.DesiredChannel, module.DesiredChannel)
				require.ErrorIs(t, module.Err, wantModule.Err)
			}
		})
	}
}

func TestTemplateLookup_GetRegularTemplates_WhenSwitchBetweenModuleVersions(t *testing.T) {
	t.Skip("This test verifies install-by-version which is not supported yet in the logic based on ModuleReleaseMeta")
	moduleToInstall := moduleToInstallByVersion("module1", version2)

	availableModuleTemplates := (&ModuleTemplateListBuilder{}).
		Add(moduleToInstall.Name, "regular", version1).
		Add(moduleToInstall.Name, "fast", version2).
		Add(moduleToInstall.Name, "experimental", version3).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version1).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version2).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version3).
		Build()

	availableModuleReleaseMetas := generateModuleReleaseMetaList(moduleToInstall.Name,
		[]v1beta2.ChannelVersionAssignment{
			{Channel: "regular", Version: version1},
			{Channel: "fast", Version: version2},
			{Channel: "experimental", Version: version3},
			{Channel: string(shared.NoneChannel), Version: version2},
		})

	fakeService := &componentdescriptor.FakeService{}
	descriptorProvider := provider.NewCachedDescriptorProvider(
		fakeService,
		descriptorcache.NewDescriptorCache(),
	)
	err := registerEmptyComponentDescriptor(fakeService, "kyma-project.io/module"+
		"/"+moduleToInstall.Name, version1)
	require.NoError(t, err)
	err = registerEmptyComponentDescriptor(fakeService, "kyma-project.io/module"+
		"/"+moduleToInstall.Name, version2)
	require.NoError(t, err)
	err = registerEmptyComponentDescriptor(fakeService, "kyma-project.io/module"+
		"/"+moduleToInstall.Name, version2)
	require.NoError(t, err)
	tests := getRegularTemplatesTestCases{
		{
			name: "When upgrade version, then result contains no error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: string(shared.NoneChannel),
					Version: version1,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantChannel: string(shared.NoneChannel),
			wantVersion: version2,
		},
		{
			name: "When downgrade version, then result contains error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: string(shared.NoneChannel),
					Version: version3,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantErrContains: versionUpgradeErr,
		},
	}

	executeGetRegularTemplatesTestCases(t, tests, availableModuleTemplates, availableModuleReleaseMetas,
		moduleToInstall, descriptorProvider)
}

func TestTemplateLookup_GetRegularTemplates_WhenSwitchFromChannelToVersion(t *testing.T) {
	t.Skip("This test verifies install-by-version which is not supported yet in the logic based on ModuleReleaseMeta")
	moduleToInstall := moduleToInstallByVersion("module1", version2)
	availableModuleTemplates := (&ModuleTemplateListBuilder{}).
		Add(moduleToInstall.Name, "regular", version1).
		Add(moduleToInstall.Name, "fast", version2).
		Add(moduleToInstall.Name, "experimental", version3).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version1).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version2).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version3).
		Build()

	availableModuleReleaseMetas := v1beta2.ModuleReleaseMetaList{}

	tests := getRegularTemplatesTestCases{
		{
			name: "When staying with the same version, then result contains no error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: "fast",
					Version: version2,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantChannel: string(shared.NoneChannel),
			wantVersion: version2,
		},
		{
			name: "When upgrade version, then result contains no error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: "regular",
					Version: version1,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantChannel: string(shared.NoneChannel),
			wantVersion: version2,
		},
		{
			name: "When downgrade version, then result contains error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: "experimental",
					Version: version3,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantErrContains: versionUpgradeErr,
		},
	}

	executeGetRegularTemplatesTestCases(t, tests, availableModuleTemplates, availableModuleReleaseMetas,
		moduleToInstall, nil)
}

func TestTemplateLookup_GetRegularTemplates_WhenSwitchFromVersionToChannel(t *testing.T) {
	moduleToInstall := testutils.NewTestModule("module1", "new_channel")
	availableModuleTemplates := (&ModuleTemplateListBuilder{}).
		Add(moduleToInstall.Name, "regular", version1).
		Add(moduleToInstall.Name, "new_channel", version2).
		Add(moduleToInstall.Name, "fast", version3).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version1).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version2).
		Add(moduleToInstall.Name, string(shared.NoneChannel), version3).
		Build()

	availableModuleReleaseMetas := generateModuleReleaseMetaList(
		moduleToInstall.Name,
		[]v1beta2.ChannelVersionAssignment{
			{Channel: "new_channel", Version: version2},
		},
	)

	fakeService := &componentdescriptor.FakeService{}
	descriptorProvider := provider.NewCachedDescriptorProvider(fakeService, descriptorcache.NewDescriptorCache())
	err := registerEmptyComponentDescriptor(fakeService, testutils.FullOCMName(moduleToInstall.Name), version2)
	require.NoError(t, err)

	tests := getRegularTemplatesTestCases{
		{
			name: "When staying with the same version, then result contains no error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: string(shared.NoneChannel),
					Version: version2,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantChannel: "new_channel",
			wantVersion: version2,
		},
		{
			name: "When upgrade version, then result contains no error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: string(shared.NoneChannel),
					Version: version1,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantChannel: "new_channel",
			wantVersion: version2,
		},
		{
			name: "When downgrade version, then result contains error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(moduleToInstall).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    moduleToInstall.Name,
					Channel: string(shared.NoneChannel),
					Version: version3,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			wantErrContains: versionUpgradeErr,
		},
	}

	executeGetRegularTemplatesTestCases(t, tests, availableModuleTemplates, availableModuleReleaseMetas,
		moduleToInstall, descriptorProvider)
}

func TestNewTemplateLookup_GetRegularTemplates_WhenModuleTemplateContainsInvalidDescriptor(t *testing.T) {
	t.Skip("This test is not relevant anymore as we no longer read ComponentDescriptor from ModuleTemplate")
	testModule := testutils.NewTestModule("module1", v1beta2.DefaultChannel)
	tests := []struct {
		name      string
		kyma      *v1beta2.Kyma
		mrmExists bool
		want      templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When module enabled in Spec, then return ModuleTemplatesByModuleName with error," +
				"with ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).Build(),
			mrmExists: true,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            provider.ErrDecode,
				},
			},
		},
		{
			name: "When module exits in ModuleStatus only, then return ModuleTemplatesByModuleName with error," +
				"with ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    testModule.Name,
					Channel: testModule.Channel,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			mrmExists: true,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            provider.ErrDecode,
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			givenTemplateList := &v1beta2.ModuleTemplateList{}
			moduleReleaseMetas := v1beta2.ModuleReleaseMetaList{}
			for _, module := range templatelookup.FetchModuleInfo(testCase.kyma) {
				givenTemplateList.Items = append(givenTemplateList.Items,
					*builder.NewModuleTemplateBuilder().
						WithName(fmt.Sprintf("%s-%s", module.Name, testModule.Version)).
						WithModuleName(module.Name).
						WithChannel(module.Channel).
						WithDescriptor(nil).
						WithRawDescriptor([]byte("{invalid_json}")).Build())

				if testCase.mrmExists {
					moduleReleaseMetas.Items = append(moduleReleaseMetas.Items,
						*builder.NewModuleReleaseMetaBuilder().
							WithModuleName(module.Name).
							WithOcmComponentName(testutils.FullOCMName(module.Name)).
							WithModuleChannelAndVersions([]v1beta2.ChannelVersionAssignment{
								{Channel: module.Channel, Version: testModule.Version},
							}).Build())
				}
			}
			reader := NewFakeModuleTemplateReader(*givenTemplateList,
				moduleReleaseMetas)
			lookup := templatelookup.NewTemplateLookup(reader,
				nil, // not used in tests
				moduletemplateinfolookup.NewLookup(reader),
				nil,
			)
			got := lookup.GetRegularTemplates(t.Context(), testCase.kyma)
			expected := len(testCase.want)
			assert.Len(t, got, expected)
			for key, module := range got {
				wantModule, ok := testCase.want[key]
				assert.True(t, ok)
				assert.Equal(t, wantModule.DesiredChannel, module.DesiredChannel)
				require.ErrorIs(t, module.Err, wantModule.Err)
			}
		})
	}
}

func TestTemplateLookup_GetRegularTemplates_WhenModuleTemplateNotFound(t *testing.T) {
	t.Skip("This test is not using ModuleReleaseMeta." +
		" It must be adjusted when the old logic, based on ModuleTemplates" +
		" is removed")
	testModule := testutils.NewTestModule("module1", v1beta2.DefaultChannel)

	fakeService := &componentdescriptor.FakeService{}
	descriptorProvider := provider.NewCachedDescriptorProvider(
		fakeService,
		descriptorcache.NewDescriptorCache(),
	)

	tests := []struct {
		name string
		kyma *v1beta2.Kyma
		want templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When no module enabled in Spec, then return empty ModuleTemplatesByModuleName",
			kyma: builder.NewKymaBuilder().Build(),
			want: templatelookup.ModuleTemplatesByModuleName{},
		},
		{
			name: "When module enabled in Spec, then return ModuleTemplatesByModuleName with error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).Build(),
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            common.ErrNoTemplatesInListResult,
				},
			},
		},
		{
			name: "When module exits in ModuleStatus only, then return ModuleTemplatesByModuleName with error",
			kyma: builder.NewKymaBuilder().
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    testModule.Name,
					Channel: testModule.Channel,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            common.ErrNoTemplatesInListResult,
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			givenTemplateList := &v1beta2.ModuleTemplateList{}
			reader := NewFakeModuleTemplateReader(*givenTemplateList,
				v1beta2.ModuleReleaseMetaList{})
			lookup := templatelookup.NewTemplateLookup(
				reader,
				descriptorProvider,
				moduletemplateinfolookup.NewLookup(reader),
				nil,
			)
			got := lookup.GetRegularTemplates(t.Context(), testCase.kyma)
			expected := len(testCase.want)
			assert.Len(t, got, expected)
			for key, module := range got {
				wantModule, ok := testCase.want[key]
				assert.True(t, ok)
				assert.Equal(t, wantModule.DesiredChannel, module.DesiredChannel)
				require.ErrorIs(t, module.Err, wantModule.Err)
				assert.Nil(t, module.ModuleTemplate)
			}
		})
	}
}

func TestTemplateLookup_GetRegularTemplates_WhenModuleTemplateExists(t *testing.T) {
	testModule := testutils.NewTestModule("module1", v1beta2.DefaultChannel)
	const moduleVersion = "1.0.0"

	fakeService := &componentdescriptor.FakeService{}
	descriptorProvider := provider.NewCachedDescriptorProvider(fakeService, descriptorcache.NewDescriptorCache())
	err := registerEmptyComponentDescriptor(fakeService, testutils.FullOCMName(testModule.Name), moduleVersion)
	require.NoError(t, err)

	tests := []struct {
		name     string
		kyma     *v1beta2.Kyma
		mrmExist bool
		want     templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When module enabled in Spec, then return expected moduleTemplateInfo, with ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).Build(),
			mrmExist: true,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            nil,
					ModuleTemplate: builder.NewModuleTemplateBuilder().
						WithModuleName(testModule.Name).
						WithVersion(moduleVersion).
						WithChannel("").
						Build(),
				},
			},
		},
		{
			name: "When module exits in ModuleStatus only, " +
				"then return expected moduleTemplateInfo, with ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    testModule.Name,
					Channel: testModule.Channel,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
					Version: "1.0.0",
				}).Build(),
			mrmExist: true,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            nil,
					ModuleTemplate: builder.NewModuleTemplateBuilder().
						WithModuleName(testModule.Name).
						WithChannel("").
						Build(),
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			givenTemplateList := &v1beta2.ModuleTemplateList{}
			moduleReleaseMetas := v1beta2.ModuleReleaseMetaList{}
			const moduleTemplateVersion = "1.0.0"
			for _, module := range templatelookup.FetchModuleInfo(testCase.kyma) {
				if testCase.mrmExist {
					givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
						WithName(fmt.Sprintf("%s-%s", module.Name, moduleTemplateVersion)).
						WithModuleName(module.Name).
						WithVersion(moduleTemplateVersion).
						Build())
					moduleReleaseMetas.Items = append(moduleReleaseMetas.Items,
						*builder.NewModuleReleaseMetaBuilder().
							WithModuleName(module.Name).
							WithOcmComponentName(testutils.FullOCMName(module.Name)).
							WithModuleChannelAndVersions([]v1beta2.ChannelVersionAssignment{
								{Channel: module.Channel, Version: moduleTemplateVersion},
							}).Build())
				} else {
					givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
						WithName(fmt.Sprintf("%s-%s", module.Name, moduleTemplateVersion)).
						WithModuleName(module.Name).
						WithVersion(moduleTemplateVersion).
						WithChannel(module.Channel).
						Build())
				}
			}
			reader := NewFakeModuleTemplateReader(*givenTemplateList,
				moduleReleaseMetas)
			lookup := templatelookup.NewTemplateLookup(
				reader,
				descriptorProvider,
				moduletemplateinfolookup.NewLookup(reader),
				nil,
			)
			expected := len(testCase.want)
			got := lookup.GetRegularTemplates(t.Context(), testCase.kyma)
			assert.Len(t, got, expected)
			for key, module := range got {
				wantModule, ok := testCase.want[key]
				assert.True(t, ok)
				assert.Equal(t, wantModule.DesiredChannel, module.DesiredChannel)
				require.ErrorIs(t, module.Err, wantModule.Err)
				if !testCase.mrmExist {
					assert.Equal(
						t,
						wantModule.Spec.Channel, //nolint:staticcheck    // legacy Channel field
						module.Spec.Channel,     //nolint:staticcheck    // legacy Channel field
					)
				}
			}
		})
	}
}

func TestTemplateNameMatch(t *testing.T) {
	targetName := "module1"

	tests := []struct {
		name     string
		template v1beta2.ModuleTemplate
		want     bool
	}{
		{
			name: "When moduleName is empty and no labels, Then return false",
			template: v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: "",
				},
			},
			want: false,
		},
		{
			name: "When moduleName is not equal to target name, Then return false",
			template: v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: "module2",
				},
			},
			want: false,
		},
		{
			name: "When moduleName is equal to target name, Then return true",
			template: v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: "module1",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := moduletemplateinfolookup.TemplateNameMatch(&tt.template, targetName); got != tt.want {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

type getRegularTemplatesTestCases []struct {
	name            string
	kyma            *v1beta2.Kyma
	wantVersion     string
	wantChannel     string
	wantErrContains string
}

func executeGetRegularTemplatesTestCases(t *testing.T,
	testCases getRegularTemplatesTestCases,
	availableModuleTemplates v1beta2.ModuleTemplateList,
	availableModuleReleaseMetas v1beta2.ModuleReleaseMetaList,
	moduleToInstall v1beta2.Module,
	descriptorProvider *provider.CachedDescriptorProvider,
) {
	t.Helper()
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reader := NewFakeModuleTemplateReader(availableModuleTemplates, availableModuleReleaseMetas)
			lookup := templatelookup.NewTemplateLookup(
				reader,
				descriptorProvider,
				moduletemplateinfolookup.NewLookup(reader),
				nil,
			)
			got := lookup.GetRegularTemplates(t.Context(), testCase.kyma)
			assert.Len(t, got, 1)
			for key, moduleTemplateInfo := range got {
				require.NotNil(t, moduleTemplateInfo)
				require.NotNil(t, moduleTemplateInfo.ModuleTemplate)
				assert.Equal(t, key, moduleToInstall.Name)
				if testCase.wantErrContains != "" {
					assert.Contains(t, moduleTemplateInfo.Err.Error(), testCase.wantErrContains)
				} else {
					assert.Equal(t, testCase.wantChannel, moduleTemplateInfo.DesiredChannel)
					assert.Equal(t, testCase.wantVersion, moduleTemplateInfo.Spec.Version)
				}
			}
		})
	}
}

func generateModuleTemplateListWithModule(moduleName, moduleChannel, moduleVersion string) v1beta2.ModuleTemplateList {
	templateList := v1beta2.ModuleTemplateList{}
	templateList.Items = append(templateList.Items, *builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName(moduleName, moduleVersion)).
		WithModuleName(moduleName).
		WithChannel(moduleChannel).
		WithVersion(moduleVersion).
		Build())
	return templateList
}

func generateModuleReleaseMetaList(moduleName string,
	channelVersions []v1beta2.ChannelVersionAssignment,
) v1beta2.ModuleReleaseMetaList {
	mrmList := v1beta2.ModuleReleaseMetaList{}
	mrmList.Items = append(mrmList.Items, *builder.NewModuleReleaseMetaBuilder().
		WithModuleName(moduleName).
		WithOcmComponentName(testutils.FullOCMName(moduleName)).
		WithModuleChannelAndVersions(channelVersions).
		Build())
	return mrmList
}

type ModuleTemplateListBuilder struct {
	ModuleTemplates []v1beta2.ModuleTemplate
}

func (mtlb *ModuleTemplateListBuilder) Add(moduleName, moduleChannel, moduleVersion string) *ModuleTemplateListBuilder {
	list := generateModuleTemplateListWithModule(moduleName, moduleChannel, moduleVersion)
	mtlb.ModuleTemplates = append(mtlb.ModuleTemplates, list.Items...)
	return mtlb
}

func (mtlb *ModuleTemplateListBuilder) Build() v1beta2.ModuleTemplateList {
	return v1beta2.ModuleTemplateList{
		Items: mtlb.ModuleTemplates,
	}
}

func moduleToInstallByVersion(moduleName, moduleVersion string) v1beta2.Module {
	return testutils.NewTestModuleWithChannelVersion(moduleName, "", moduleVersion)
}

// registerEmptyComponentDescriptor registers an (almost) empty component descriptor with the given name and version.
func registerEmptyComponentDescriptor(fakeService *componentdescriptor.FakeService, name, version string) error {
	cd := compdesc.New(name, version)
	cdBytes, err := compdesc.Encode(cd)
	if err != nil {
		return err
	}
	fakeService.RegisterWithNameVersionOverride(name, version, cdBytes)
	return nil
}
