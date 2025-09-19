//nolint:testpackage // this file tests unexported types of the package
package remote

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// TestSyncer_SyncToSKR_happypath tests the happy path of the SyncToSKR method,
// with some ModuleTemplates to be installed in the SKR and some modules to be deleted from the SKR.
func TestSyncer_SyncToSKR_happypath(t *testing.T) { //nolint:dupl,revive // duplication will be removed: https://github.com/kyma-project/lifecycle-manager/issues/2015
	// given
	mtKCP1 := moduleTemplate("mt1", "kcp-system") // this one should be installed in the SKR, because it's not there
	mtKCP2 := moduleTemplate("mt2", "kcp-system")
	mtKCP3 := moduleTemplate("mt3", "kcp-system")

	mtSKR2 := moduleTemplate("mt2", "kyma-system")
	mtSKR3 := moduleTemplate("mt3", "kyma-system")
	mtSKR4 := moduleTemplate("mt4", "kyma-system") // this one should be deleted, because it's not in the KCP

	// Create a fake client with the SKR modules
	scheme, err := v1beta2.SchemeBuilder.Build()
	require.NoError(t, err)
	skrClient := fake.NewClientBuilder().
		WithObjects(&mtSKR2, &mtSKR3, &mtSKR4).
		WithScheme(scheme).
		Build()

	onSyncConcurrentlyFn := func(_ context.Context, kcpModules []v1beta2.ModuleTemplate) {
		if len(kcpModules) != 3 {
			t.Errorf("Expected 3 kcp modules, got %d", len(kcpModules))
		}
		if kcpModules[0].Name != "mt1" {
			t.Errorf("Expected module mt1, got %s", kcpModules[0].Name)
		}
		if kcpModules[1].Name != "mt2" {
			t.Errorf("Expected module mt2, got %s", kcpModules[1].Name)
		}
		if kcpModules[2].Name != "mt3" {
			t.Errorf("Expected module mt3, got %s", kcpModules[2].Name)
		}
	}

	onDeleteConcurrentlyFn := func(_ context.Context, runtimeModules []v1beta2.ModuleTemplate) {
		if len(runtimeModules) != 1 {
			t.Errorf("Expected 1 runtime module, got %d", len(runtimeModules))
		}
		if runtimeModules[0].Name != "mt4" {
			t.Errorf("Expected module mt4, got %s", runtimeModules[0].Name)
		}
	}

	syncWokerFactoryFn := func(kcpClient, skrClient client.Client, settings *Settings) moduleTemplateSyncWorker {
		return &fakeSyncWorker{
			namespace:            settings.Namespace,
			onSyncConcurrently:   onSyncConcurrentlyFn,
			onDeleteConcurrently: onDeleteConcurrentlyFn,
		}
	}

	subject := moduleTemplateSyncer{
		skrClient:           skrClient,
		settings:            getSettings(),
		syncWorkerFactoryFn: syncWokerFactoryFn,
	}

	// when
	err = subject.SyncToSKR(t.Context(), []v1beta2.ModuleTemplate{mtKCP1, mtKCP2, mtKCP3})

	// then
	assert.NoError(t, err)
}

// TestSyncer_SyncToSKR_nilList tests the case when the list of KCP modules is nil.
func TestSyncer_SyncToSKR_nilList(t *testing.T) {
	// given
	mtSKR2 := moduleTemplate("mt2", "kyma-system") // should be deleted, because it's not in the KCP
	mtSKR3 := moduleTemplate("mt3", "kyma-system") // should be deleted, because it's not in the KCP
	mtSKR4 := moduleTemplate("mt4", "kyma-system") // should be deleted, because it's not in the KCP

	// Create a fake client with the SKR modules
	scheme, err := v1beta2.SchemeBuilder.Build()
	require.NoError(t, err)
	skrClient := fake.NewClientBuilder().
		WithObjects(&mtSKR2, &mtSKR3, &mtSKR4).
		WithScheme(scheme).
		Build()

	// onSyncConcurrentlyFn "pretends" to be the moduleTemplateConcurrentWorker.SyncConcurrently
	onSyncConcurrentlyFn := func(_ context.Context, kcpModules []v1beta2.ModuleTemplate) {
		if kcpModules != nil {
			t.Errorf("Expected nil kcp modules, got %v", kcpModules)
		}
	}

	// onDeleteConcurrentlyFn "pretends" to be the moduleTemplateConcurrentWorker.DeleteConcurrently
	onDeleteConcurrentlyFn := func(_ context.Context, runtimeModules []v1beta2.ModuleTemplate) {
		if len(runtimeModules) != 3 {
			t.Errorf("Expected 3 runtime module, got %d", len(runtimeModules))
		}
		if runtimeModules[0].Name != "mt2" {
			t.Errorf("Expected module mt2, got %s", runtimeModules[0].Name)
		}
		if runtimeModules[1].Name != "mt3" {
			t.Errorf("Expected module mt2, got %s", runtimeModules[1].Name)
		}
		if runtimeModules[2].Name != "mt4" {
			t.Errorf("Expected module mt2, got %s", runtimeModules[2].Name)
		}
	}

	syncWokerFactoryFn := func(kcpClient, skrClient client.Client, settings *Settings) moduleTemplateSyncWorker {
		return &fakeSyncWorker{
			namespace:            settings.Namespace,
			onSyncConcurrently:   onSyncConcurrentlyFn,
			onDeleteConcurrently: onDeleteConcurrentlyFn,
		}
	}

	subject := moduleTemplateSyncer{
		skrClient:           skrClient,
		settings:            getSettings(),
		syncWorkerFactoryFn: syncWokerFactoryFn,
	}

	// when
	var nilModuleTemplateList []v1beta2.ModuleTemplate = nil
	err = subject.SyncToSKR(t.Context(), nilModuleTemplateList)

	// then
	assert.NoError(t, err)
}

func moduleTemplate(name, namespace string) v1beta2.ModuleTemplate {
	return v1beta2.ModuleTemplate{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			ManagedFields: []apimetav1.ManagedFieldsEntry{
				{
					Manager: moduleCatalogSyncFieldManager,
				},
			},
		},
	}
}

func getSettings() *Settings {
	force := true
	return &Settings{
		Namespace:       "kyma-system",
		SSAPatchOptions: &client.PatchOptions{FieldManager: moduleCatalogSyncFieldManager, Force: &force},
	}
}

// Implements the syncWorker interface.
type fakeSyncWorker struct {
	namespace            string
	onSyncConcurrently   func(ctx context.Context, kcpModules []v1beta2.ModuleTemplate)
	onDeleteConcurrently func(ctx context.Context, runtimeModules []v1beta2.ModuleTemplate)
}

func (f *fakeSyncWorker) SyncConcurrently(ctx context.Context, kcpModules []v1beta2.ModuleTemplate) error {
	f.onSyncConcurrently(ctx, kcpModules)

	// Simulate namespace switch on modules in kcpModules list that happens in
	// moduleTemplateConcurrentWorker.SyncConcurrently
	// This is necessary for proper diff calculation later in the process.
	for i := range kcpModules {
		prepareModuleTemplateForSSA(&kcpModules[i], f.namespace)
	}

	return nil
}

func (f *fakeSyncWorker) DeleteConcurrently(ctx context.Context, runtimeModules []v1beta2.ModuleTemplate) error {
	f.onDeleteConcurrently(ctx, runtimeModules)
	return nil
}
