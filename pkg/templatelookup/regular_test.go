package templatelookup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

// TestSetup provides a simplified way to create test scenarios for templatelookup
type TestSetup struct {
	moduleTemplate    *v1beta2.ModuleTemplate
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta
	kyma              *v1beta2.Kyma
	client            client.Client
}

// NewTestSetup creates a new test setup with sensible defaults
func NewTestSetup() *TestSetup {
	scheme := machineryruntime.NewScheme()
	_ = v1beta2.AddToScheme(scheme)

	return &TestSetup{
		client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}
}

// WithMandatoryModule sets up a mandatory module scenario
func (ts *TestSetup) WithMandatoryModule(moduleName, version string) *TestSetup {
	ts.moduleTemplate = builder.NewModuleTemplateBuilder().
		WithName(moduleName + "-" + version).
		WithNamespace("test-namespace").
		WithModuleName(moduleName).
		WithVersion(version).
		Build()

	ts.moduleReleaseMeta = builder.NewModuleReleaseMetaBuilder().
		WithName(moduleName).
		WithNamespace("test-namespace").
		WithModuleName(moduleName).
		WithMandatory(version).
		Build()

	ts.kyma = builder.NewKymaBuilder().
		WithNamespace("test-namespace").
		Build()

	return ts
}

// WithChannelModule sets up a channel-based module scenario
func (ts *TestSetup) WithChannelModule(moduleName, channel, version string) *TestSetup {
	ts.moduleTemplate = builder.NewModuleTemplateBuilder().
		WithName(moduleName + "-" + version).
		WithNamespace("test-namespace").
		WithModuleName(moduleName).
		WithVersion(version).
		Build()

	ts.moduleReleaseMeta = builder.NewModuleReleaseMetaBuilder().
		WithName(moduleName).
		WithNamespace("test-namespace").
		WithModuleName(moduleName).
		WithSingleModuleChannelAndVersions(channel, version).
		Build()

	ts.kyma = builder.NewKymaBuilder().
		WithNamespace("test-namespace").
		WithChannel(channel).
		Build()

	return ts
}

// WithMissingTemplate creates a scenario where the module template doesn't exist
func (ts *TestSetup) WithMissingTemplate(moduleName, channel, version string) *TestSetup {
	// Only create ModuleReleaseMeta, no ModuleTemplate
	ts.moduleReleaseMeta = builder.NewModuleReleaseMetaBuilder().
		WithName(moduleName).
		WithNamespace("test-namespace").
		WithModuleName(moduleName).
		WithSingleModuleChannelAndVersions(channel, version).
		Build()

	ts.kyma = builder.NewKymaBuilder().
		WithNamespace("test-namespace").
		WithChannel(channel).
		Build()

	return ts
}

// Build creates the client with all configured objects and returns it
func (ts *TestSetup) Build() client.Client {
	scheme := machineryruntime.NewScheme()
	_ = v1beta2.AddToScheme(scheme)

	objects := make([]client.Object, 0)
	if ts.moduleTemplate != nil {
		objects = append(objects, ts.moduleTemplate)
	}
	if ts.moduleReleaseMeta != nil {
		objects = append(objects, ts.moduleReleaseMeta)
	}
	if ts.kyma != nil {
		objects = append(objects, ts.kyma)
	}

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

// GetModuleInfo returns a ModuleInfo for testing
func (ts *TestSetup) GetModuleInfo(moduleName string) *templatelookup.ModuleInfo {
	return &templatelookup.ModuleInfo{
		Module: v1beta2.Module{
			Name: moduleName,
		},
	}
}

// GetKyma returns the configured Kyma object
func (ts *TestSetup) GetKyma() *v1beta2.Kyma {
	return ts.kyma
}

// GetModuleReleaseMeta returns the configured ModuleReleaseMeta
func (ts *TestSetup) GetModuleReleaseMeta() *v1beta2.ModuleReleaseMeta {
	return ts.moduleReleaseMeta
}

// ExecuteLookupModuleTemplate is a simplified test helper for LookupModuleTemplate
func ExecuteLookupModuleTemplate(t *testing.T, setup *TestSetup, moduleName string) templatelookup.ModuleTemplateInfo {
	client := setup.Build()
	moduleInfo := setup.GetModuleInfo(moduleName)
	kyma := setup.GetKyma()
	moduleReleaseMeta := setup.GetModuleReleaseMeta()

	return templatelookup.LookupModuleTemplate(t.Context(), client, moduleInfo, kyma, moduleReleaseMeta)
}

// TestLookupModuleTemplate covers the core LookupModuleTemplate functionality
func TestLookupModuleTemplate(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *TestSetup
		moduleName  string
		expectError bool
		expectNil   bool
	}{
		{
			name: "mandatory module succeeds",
			setupFunc: func() *TestSetup {
				return NewTestSetup().WithMandatoryModule("test-module", "1.0.0")
			},
			moduleName:  "test-module",
			expectError: false,
			expectNil:   false,
		},
		{
			name: "channel module succeeds",
			setupFunc: func() *TestSetup {
				return NewTestSetup().WithChannelModule("test-module", "stable", "1.0.0")
			},
			moduleName:  "test-module",
			expectError: false,
			expectNil:   false,
		},
		{
			name: "missing template fails gracefully",
			setupFunc: func() *TestSetup {
				return NewTestSetup().WithMissingTemplate("test-module", "stable", "1.0.0")
			},
			moduleName:  "test-module",
			expectError: true,
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := tt.setupFunc()
			result := ExecuteLookupModuleTemplate(t, setup, tt.moduleName)

			if tt.expectError {
				assert.Error(t, result.Err)
			} else {
				assert.NoError(t, result.Err)
			}

			if tt.expectNil {
				assert.Nil(t, result.ModuleTemplate)
			} else {
				assert.NotNil(t, result.ModuleTemplate)
				assert.Contains(t, result.ModuleTemplate.Name, tt.moduleName)
			}
		})
	}
}

// TestValidateTemplateMode covers template validation logic
func TestValidateTemplateMode(t *testing.T) {
	tests := []struct {
		name     string
		template templatelookup.ModuleTemplateInfo
		kyma     *v1beta2.Kyma
		wantErr  error
	}{
		{
			name: "error propagation works",
			template: templatelookup.ModuleTemplateInfo{
				Err: templatelookup.ErrTemplateNotAllowed,
			},
			wantErr: templatelookup.ErrTemplateNotAllowed,
		},
		{
			name: "internal module blocked for non-internal kyma",
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
			name: "beta module blocked for non-beta kyma",
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
			name: "mandatory module blocked",
			template: templatelookup.ModuleTemplateInfo{
				ModuleTemplate: builder.NewModuleTemplateBuilder().
					WithMandatory(true).Build(),
			},
			kyma:    builder.NewKymaBuilder().Build(),
			wantErr: common.ErrNoTemplatesInListResult,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := templatelookup.ValidateTemplateMode(tt.template, tt.kyma)
			require.ErrorIs(t, result.Err, tt.wantErr)
		})
	}
}

// TestTemplateNameMatch covers the template name matching utility
func TestTemplateNameMatch(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		targetName   string
		want         bool
	}{
		{
			name:         "empty name returns false",
			templateName: "",
			targetName:   "module1",
			want:         false,
		},
		{
			name:         "different names return false",
			templateName: "module2",
			targetName:   "module1",
			want:         false,
		},
		{
			name:         "matching names return true",
			templateName: "module1",
			targetName:   "module1",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: tt.templateName,
				},
			}
			result := templatelookup.TemplateNameMatch(&template, tt.targetName)
			assert.Equal(t, tt.want, result)
		})
	}
}
