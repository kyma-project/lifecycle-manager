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
	ocmmetav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
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
			if got := templatelookup.ValidateTemplateMode(testCase.template, testCase.kyma); !errors.Is(got.Err,
				testCase.wantErr) {
				t.Errorf("ValidateTemplateMode() = %v, want %v", got, testCase.wantErr)
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
				provider.NewCachedDescriptorProvider(),
				moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
					[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
						moduletemplateinfolookup.NewByVersionStrategy(nil),
						moduletemplateinfolookup.NewByChannelStrategy(nil),
						moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(nil),
					},
				),
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

	tests := []struct {
		name                       string
		kyma                       *v1beta2.Kyma
		availableModuleTemplate    v1beta2.ModuleTemplateList
		availableModuleReleaseMeta v1beta2.ModuleReleaseMetaList
		want                       templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When upgrade version during channel switch, " +
				"then result contains no error, without ModuleReleaseMeta",
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
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, testModule.Channel,
				version2),
			availableModuleReleaseMeta: v1beta2.ModuleReleaseMetaList{},
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            nil,
				},
			},
		},
		{
			name: "When downgrade version during channel switch, Then result contains error, without ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).
				WithModuleStatus(v1beta2.ModuleStatus{
					Name:    testModule.Name,
					Channel: v1beta2.DefaultChannel,
					Version: version2,
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, testModule.Channel,
				version1),
			availableModuleReleaseMeta: v1beta2.ModuleReleaseMetaList{},

			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            templatelookup.ErrTemplateUpdateNotAllowed,
				},
			},
		},
		{
			name: "When upgrade version during channel switch, Then result contains no error, with ModuleReleaseMeta",
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
			name: "When downgrade version during channel switch, Then result contains error, with ModuleReleaseMeta",
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
			reader := NewFakeModuleTemplateReader(testCase.availableModuleTemplate, testCase.availableModuleReleaseMeta)
			lookup := templatelookup.NewTemplateLookup(
				reader,
				provider.NewCachedDescriptorProvider(),
				moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
					[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
						moduletemplateinfolookup.NewByVersionStrategy(reader),
						moduletemplateinfolookup.NewByChannelStrategy(reader),
						moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(reader),
					},
				),
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
		moduleToInstall)
}

func TestTemplateLookup_GetRegularTemplates_WhenSwitchFromChannelToVersion(t *testing.T) {
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
		moduleToInstall)
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

	availableModuleReleaseMetas := v1beta2.ModuleReleaseMetaList{}

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
		moduleToInstall)
}

func TestNewTemplateLookup_GetRegularTemplates_WhenModuleTemplateContainsInvalidDescriptor(t *testing.T) {
	testModule := testutils.NewTestModule("module1", v1beta2.DefaultChannel)
	tests := []struct {
		name      string
		kyma      *v1beta2.Kyma
		mrmExists bool
		want      templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When module enabled in Spec, then return ModuleTemplatesByModuleName with error, " +
				"without ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).Build(),
			mrmExists: false,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            provider.ErrDecode,
				},
			},
		},
		{
			name: "When module exits in ModuleStatus only, then return ModuleTemplatesByModuleName with error," +
				"without ModuleReleaseMeta",
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
			mrmExists: false,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            provider.ErrDecode,
				},
			},
		},
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
				givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
					WithName(fmt.Sprintf("%s-%s", module.Name, testModule.Version)).
					WithModuleName(module.Name).
					WithLabelModuleName(module.Name).
					WithChannel(module.Channel).
					WithDescriptor(nil).
					WithRawDescriptor([]byte("{invalid_json}")).Build())

				if testCase.mrmExists {
					moduleReleaseMetas.Items = append(moduleReleaseMetas.Items,
						*builder.NewModuleReleaseMetaBuilder().
							WithModuleName(module.Name).
							WithModuleChannelAndVersions([]v1beta2.ChannelVersionAssignment{
								{Channel: module.Channel, Version: testModule.Version},
							}).Build())
				}
			}
			reader := NewFakeModuleTemplateReader(*givenTemplateList,
				moduleReleaseMetas)
			lookup := templatelookup.NewTemplateLookup(
				reader,
				provider.NewCachedDescriptorProvider(),
				moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
					[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
						moduletemplateinfolookup.NewByVersionStrategy(reader),
						moduletemplateinfolookup.NewByChannelStrategy(reader),
						moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(reader),
					},
				),
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
	testModule := testutils.NewTestModule("module1", v1beta2.DefaultChannel)

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
				provider.NewCachedDescriptorProvider(),
				moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
					[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
						moduletemplateinfolookup.NewByVersionStrategy(reader),
						moduletemplateinfolookup.NewByChannelStrategy(reader),
						moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(reader),
					},
				),
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

	tests := []struct {
		name     string
		kyma     *v1beta2.Kyma
		mrmExist bool
		want     templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When module enabled in Spec, then return expected moduleTemplateInfo, without ModuleReleaseMeta",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).Build(),
			mrmExist: false,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            nil,
					ModuleTemplate: builder.NewModuleTemplateBuilder().
						WithModuleName(testModule.Name).
						WithChannel(testModule.Channel).
						Build(),
				},
			},
		},
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
						WithChannel("").
						Build(),
				},
			},
		},
		{
			name: "When module exits in ModuleStatus only, " +
				"then return expected moduleTemplateInfo, without ModuleReleaseMeta",
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
			mrmExist: false,
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            nil,
					ModuleTemplate: builder.NewModuleTemplateBuilder().
						WithModuleName(testModule.Name).
						WithChannel(testModule.Channel).
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
						WithOCM(compdescv2.SchemaVersion).Build())
					moduleReleaseMetas.Items = append(moduleReleaseMetas.Items,
						*builder.NewModuleReleaseMetaBuilder().
							WithModuleName(module.Name).
							WithModuleChannelAndVersions([]v1beta2.ChannelVersionAssignment{
								{Channel: module.Channel, Version: moduleTemplateVersion},
							}).Build())
				} else {
					givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
						WithName(fmt.Sprintf("%s-%s", module.Name, moduleTemplateVersion)).
						WithAnnotation(shared.ModuleVersionAnnotation, moduleTemplateVersion).
						WithLabelModuleName(module.Name).
						WithChannel(module.Channel).
						WithOCM(compdescv2.SchemaVersion).Build())
				}
			}
			reader := NewFakeModuleTemplateReader(*givenTemplateList,
				moduleReleaseMetas)
			lookup := templatelookup.NewTemplateLookup(
				reader,
				provider.NewCachedDescriptorProvider(),
				moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
					[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
						moduletemplateinfolookup.NewByVersionStrategy(reader),
						moduletemplateinfolookup.NewByChannelStrategy(reader),
						moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(reader),
					},
				),
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
					assert.Equal(t, wantModule.Spec.Channel, module.Spec.Channel)
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
		{
			name: "When moduleName is empty but legacy label matches,  Then return true",
			template: v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ModuleName: "module1",
					},
				},
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: "",
				},
			},
			want: true,
		},
		{
			name: "When moduleName does not match and legacy label matches, " +
				"Then return false as moduleName takes precedence over label",
			template: v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ModuleName: "module1",
					},
				},
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: "module2",
				},
			},
			want: false,
		},
		{
			name: "When moduleName does matches and legacy label does not match," +
				" Then return true as moduleName takes precedence over label",
			template: v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ModuleName: "module2",
					},
				},
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
) {
	t.Helper()
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reader := NewFakeModuleTemplateReader(availableModuleTemplates, availableModuleReleaseMetas)
			lookup := templatelookup.NewTemplateLookup(
				reader,
				provider.NewCachedDescriptorProvider(),
				moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
					[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
						moduletemplateinfolookup.NewByVersionStrategy(reader),
						moduletemplateinfolookup.NewByChannelStrategy(reader),
						moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(reader),
					},
				),
			)
			got := lookup.GetRegularTemplates(t.Context(), testCase.kyma)
			assert.Len(t, got, 1)
			for key, module := range got {
				assert.Equal(t, key, moduleToInstall.Name)
				if testCase.wantErrContains != "" {
					assert.Contains(t, module.Err.Error(), testCase.wantErrContains)
				} else {
					assert.Equal(t, testCase.wantChannel, module.DesiredChannel)
					assert.Equal(t, testCase.wantVersion, module.Spec.Version)
				}
			}
		})
	}
}

func generateModuleTemplateListWithModule(moduleName, moduleChannel, moduleVersion string) v1beta2.ModuleTemplateList {
	templateList := v1beta2.ModuleTemplateList{}
	templateList.Items = append(templateList.Items, *builder.NewModuleTemplateBuilder().
		WithName(fmt.Sprintf("%s-%s", moduleName, moduleVersion)).
		WithModuleName(moduleName).
		WithLabelModuleName(moduleName).
		WithChannel(moduleChannel).
		WithVersion(moduleVersion).
		WithDescriptor(&types.Descriptor{
			ComponentDescriptor: &compdesc.ComponentDescriptor{
				Metadata: compdesc.Metadata{
					ConfiguredVersion: compdescv2.SchemaVersion,
				},
				ComponentSpec: compdesc.ComponentSpec{
					ObjectMeta: ocmmetav1.ObjectMeta{
						Version: moduleVersion,
					},
				},
			},
		}).Build())
	return templateList
}

func generateModuleReleaseMetaList(moduleName string,
	channelVersions []v1beta2.ChannelVersionAssignment,
) v1beta2.ModuleReleaseMetaList {
	mrmList := v1beta2.ModuleReleaseMetaList{}
	mrmList.Items = append(mrmList.Items, *builder.NewModuleReleaseMetaBuilder().
		WithModuleName(moduleName).
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
