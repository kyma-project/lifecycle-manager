package templatelookup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_GetRegularTemplates_WhenInvalidModuleProvided(t *testing.T) {
	tests := []struct {
		name       string
		KymaSpec   v1beta2.KymaSpec
		KymaStatus v1beta2.KymaStatus
		wantErr    error
	}{
		{
			name: "When Module in Spec contains both Channel and Version, Then the output is empty",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Channel: "regular", Version: "v1.0"},
				},
			},
			wantErr: templatelookup.ErrTemplateNotValid,
		},
		{
			name: "When Template not exists in Status, Then the output is empty",
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
			wantErr: templatelookup.ErrTemplateNotValid,
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(tt.name, func(t *testing.T) {
			lookup := &templatelookup.TemplateLookup{
				Reader:             nil,
				DescriptorProvider: provider.NewCachedDescriptorProvider(nil),
			}
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

func TestFilterTemplate(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := templatelookup.FilterTemplate(tt.template, tt.kyma,
				provider.NewCachedDescriptorProvider(nil)); !errors.Is(got.Err, tt.wantErr) {
				t.Errorf("FilterTemplate() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}
