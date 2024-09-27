package templatelookup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
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
	templateList v1beta2.ModuleTemplateList
}

func NewFakeModuleTemplateReader(templateList v1beta2.ModuleTemplateList) *FakeModuleTemplateReader {
	return &FakeModuleTemplateReader{
		templateList: templateList,
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

func (f *FakeModuleTemplateReader) Get(_ context.Context, _ client.ObjectKey, _ client.Object,
	_ ...client.GetOption,
) error {
	return nil
}

func TestValidateTemplateMode(t *testing.T) {
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
			lookup := templatelookup.NewTemplateLookup(nil, provider.NewCachedDescriptorProvider())
			kyma := &v1beta2.Kyma{
				Spec:   test.KymaSpec,
				Status: test.KymaStatus,
			}
			got := lookup.GetRegularTemplates(context.TODO(), kyma)
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
		name                    string
		kyma                    *v1beta2.Kyma
		availableModuleTemplate v1beta2.ModuleTemplateList
		want                    templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When upgrade version during channel switch, Then result contains no error",
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
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, testModule.Channel, version2),
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            nil,
				},
			},
		}, {
			name: "When downgrade version during channel switch, Then result contains error",
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
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, testModule.Channel, version1),
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
			lookup := templatelookup.NewTemplateLookup(NewFakeModuleTemplateReader(testCase.availableModuleTemplate),
				provider.NewCachedDescriptorProvider())
			got := lookup.GetRegularTemplates(context.TODO(), testCase.kyma)
			assert.Equal(t, len(got), len(testCase.want))
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

	tests := []struct {
		name            string
		kyma            *v1beta2.Kyma
		wantVersion     string
		wantChannel     string
		wantErrContains string
	}{
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			lookup := templatelookup.NewTemplateLookup(NewFakeModuleTemplateReader(availableModuleTemplates),
				provider.NewCachedDescriptorProvider())
			got := lookup.GetRegularTemplates(context.TODO(), testCase.kyma)
			assert.Len(t, got, 1)
			for key, module := range got {
				assert.Equal(t, key, moduleToInstall.Name)
				if testCase.wantErrContains != "" {
					assert.Contains(t, module.Err.Error(), testCase.wantErrContains)
				} else {
					assert.Equal(t, testCase.wantChannel, module.DesiredChannel)
					assert.Equal(t, testCase.wantVersion, module.ModuleTemplate.Spec.Version)
				}
			}
		})
	}
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

	tests := []struct {
		name            string
		kyma            *v1beta2.Kyma
		wantVersion     string
		wantChannel     string
		wantErrContains string
	}{
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			lookup := templatelookup.NewTemplateLookup(NewFakeModuleTemplateReader(availableModuleTemplates),
				provider.NewCachedDescriptorProvider())
			got := lookup.GetRegularTemplates(context.TODO(), testCase.kyma)
			assert.Len(t, got, 1)
			for key, module := range got {
				assert.Equal(t, key, moduleToInstall.Name)
				if testCase.wantErrContains != "" {
					assert.Contains(t, module.Err.Error(), testCase.wantErrContains)
				} else {
					assert.Equal(t, testCase.wantChannel, module.DesiredChannel)
					assert.Equal(t, testCase.wantVersion, module.ModuleTemplate.Spec.Version)
				}
			}
		})
	}
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

	tests := []struct {
		name            string
		kyma            *v1beta2.Kyma
		wantVersion     string
		wantChannel     string
		wantErrContains string
	}{
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			lookup := templatelookup.NewTemplateLookup(NewFakeModuleTemplateReader(availableModuleTemplates),
				provider.NewCachedDescriptorProvider())
			got := lookup.GetRegularTemplates(context.TODO(), testCase.kyma)
			assert.Len(t, got, 1)
			for key, module := range got {
				assert.Equal(t, key, moduleToInstall.Name)
				if testCase.wantErrContains != "" {
					assert.Contains(t, module.Err.Error(), testCase.wantErrContains)
				} else {
					assert.Equal(t, testCase.wantChannel, module.DesiredChannel)
					assert.Equal(t, testCase.wantVersion, module.ModuleTemplate.Spec.Version)
				}
			}
		})
	}
}

func TestNewTemplateLookup_GetRegularTemplates_WhenModuleTemplateContainsInvalidDescriptor(t *testing.T) {
	testModule := testutils.NewTestModule("module1", v1beta2.DefaultChannel)

	tests := []struct {
		name string
		kyma *v1beta2.Kyma
		want templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When module enabled in Spec, then return ModuleTemplatesByModuleName with error",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).Build(),
			want: templatelookup.ModuleTemplatesByModuleName{
				testModule.Name: &templatelookup.ModuleTemplateInfo{
					DesiredChannel: testModule.Channel,
					Err:            provider.ErrDecode,
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
					Err:            provider.ErrDecode,
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			givenTemplateList := &v1beta2.ModuleTemplateList{}
			for _, module := range templatelookup.FindAvailableModules(testCase.kyma) {
				givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
					WithModuleName(module.Name).
					WithLabelModuleName(module.Name).
					WithChannel(module.Channel).
					WithDescriptor(nil).
					WithRawDescriptor([]byte("{invalid_json}")).Build())
			}
			lookup := templatelookup.NewTemplateLookup(NewFakeModuleTemplateReader(*givenTemplateList),
				provider.NewCachedDescriptorProvider())
			got := lookup.GetRegularTemplates(context.TODO(), testCase.kyma)
			assert.Equal(t, len(got), len(testCase.want))
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
					Err:            templatelookup.ErrNoTemplatesInListResult,
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
					Err:            templatelookup.ErrNoTemplatesInListResult,
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			givenTemplateList := &v1beta2.ModuleTemplateList{}
			lookup := templatelookup.NewTemplateLookup(NewFakeModuleTemplateReader(*givenTemplateList),
				provider.NewCachedDescriptorProvider())
			got := lookup.GetRegularTemplates(context.TODO(), testCase.kyma)
			assert.Equal(t, len(got), len(testCase.want))
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
		name string
		kyma *v1beta2.Kyma
		want templatelookup.ModuleTemplatesByModuleName
	}{
		{
			name: "When module enabled in Spec, then return expected moduleTemplateInfo",
			kyma: builder.NewKymaBuilder().
				WithEnabledModule(testModule).Build(),
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
			name: "When module exits in ModuleStatus only, then return expected moduleTemplateInfo",
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
				}).Build(),
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
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			givenTemplateList := &v1beta2.ModuleTemplateList{}
			for _, module := range templatelookup.FindAvailableModules(testCase.kyma) {
				givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
					WithModuleName(module.Name).
					WithLabelModuleName(module.Name).
					WithChannel(module.Channel).
					WithOCM(compdescv2.SchemaVersion).Build())
			}
			lookup := templatelookup.NewTemplateLookup(NewFakeModuleTemplateReader(*givenTemplateList),
				provider.NewCachedDescriptorProvider())
			got := lookup.GetRegularTemplates(context.TODO(), testCase.kyma)
			assert.Equal(t, len(got), len(testCase.want))
			for key, module := range got {
				wantModule, ok := testCase.want[key]
				assert.True(t, ok)
				assert.Equal(t, wantModule.DesiredChannel, module.DesiredChannel)
				require.ErrorIs(t, module.Err, wantModule.Err)
				assert.Equal(t, wantModule.ModuleTemplate.Spec.Channel, module.ModuleTemplate.Spec.Channel)
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
			name: "When moduleName does not match and legacy label matches, Then return false as moduleName takes precedence over label",
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
			name: "When moduleName does matches and legacy label does not match, Then return true as moduleName takes precedence over label",
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
			if got := templatelookup.TemplateNameMatch(&tt.template, targetName); got != tt.want {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func generateModuleTemplateListWithModule(moduleName, moduleChannel, moduleVersion string) v1beta2.ModuleTemplateList {
	templateList := v1beta2.ModuleTemplateList{}
	templateList.Items = append(templateList.Items, *builder.NewModuleTemplateBuilder().
		WithModuleName(moduleName).
		WithLabelModuleName(moduleName).
		WithChannel(moduleChannel).
		WithVersion(moduleVersion).
		WithDescriptor(&v1beta2.Descriptor{
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
