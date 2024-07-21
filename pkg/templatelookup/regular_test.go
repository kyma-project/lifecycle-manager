package templatelookup_test

import (
	"context"
	"errors"
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

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
				provider.NewCachedDescriptorProvider()); !errors.Is(got.Err, tt.wantErr) {
				t.Errorf("FilterTemplate() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}

func TestMarkInvalidChannelSkewUpdate(t *testing.T) {
	tests := []struct {
		name                  string
		moduleTemplate        *templatelookup.ModuleTemplateInfo
		moduleTemplateVersion string
		moduleStatus          *v1beta2.ModuleStatus
		wantErr               error
	}{
		{
			name: "When upgrade version during channel switch, Then result contains no error",
			moduleTemplate: &templatelookup.ModuleTemplateInfo{
				ModuleTemplate: &v1beta2.ModuleTemplate{Spec: v1beta2.ModuleTemplateSpec{Channel: "fast"}},
			},
			moduleTemplateVersion: "1.1.0",
			moduleStatus: &v1beta2.ModuleStatus{
				Channel:  "regular",
				Version:  "1.0.0",
				Template: &v1beta2.TrackingObject{TypeMeta: apimetav1.TypeMeta{Kind: "ModuleTemplate"}},
			},
			wantErr: nil,
		}, {
			name: "When downgrade version during channel switch, Then result contains error",
			moduleTemplate: &templatelookup.ModuleTemplateInfo{
				ModuleTemplate: &v1beta2.ModuleTemplate{Spec: v1beta2.ModuleTemplateSpec{Channel: "fast"}},
			},
			moduleTemplateVersion: "1.0.0",
			moduleStatus: &v1beta2.ModuleStatus{
				Channel:  "regular",
				Version:  "1.1.0",
				Template: &v1beta2.TrackingObject{TypeMeta: apimetav1.TypeMeta{Kind: "ModuleTemplate"}},
			},
			wantErr: templatelookup.ErrTemplateUpdateNotAllowed,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			templatelookup.MarkInvalidChannelSkewUpdate(context.TODO(), testCase.moduleTemplate, testCase.moduleStatus,
				testCase.moduleTemplateVersion)
		})
		if !errors.Is(testCase.moduleTemplate.Err, testCase.wantErr) {
			t.Errorf("MarkInvalidChannelSkewUpdate() = %v, want %v", testCase.moduleTemplate.Err, testCase.wantErr)
		}
	}
}
