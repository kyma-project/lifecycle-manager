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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
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
					Version: "1.0.0",
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, testModule.Channel, "1.1.0"),
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
					Version: "1.1.0",
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: 1,
						},
					},
				}).Build(),
			availableModuleTemplate: generateModuleTemplateListWithModule(testModule.Name, testModule.Channel, "1.0.0"),
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

func generateModuleTemplateListWithModule(moduleName, moduleChannel, moduleVersion string) v1beta2.ModuleTemplateList {
	templateList := v1beta2.ModuleTemplateList{}
	templateList.Items = append(templateList.Items, *builder.NewModuleTemplateBuilder().
		WithModuleName(moduleName).
		WithChannel(moduleChannel).
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
			for _, module := range testCase.kyma.GetAvailableModules() {
				givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
					WithModuleName(module.Name).
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
			for _, module := range testCase.kyma.GetAvailableModules() {
				givenTemplateList.Items = append(givenTemplateList.Items, *builder.NewModuleTemplateBuilder().
					WithModuleName(module.Name).
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
