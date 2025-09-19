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

// TestModuleReleaseMetaSyncer_SyncToSKR_happypath tests the happy path of the SyncToSKR method,
// with some ModuleReleaseMetas to be installed in the SKR and some objects to be deleted from the SKR.
func TestModuleReleaseMetaSyncer_SyncToSKR_happypath(t *testing.T) { //nolint:dupl,revive // duplication will be removed: https://github.com/kyma-project/lifecycle-manager/issues/2015
	// given
	mrmKCP1 := moduleReleaseMeta(
		"mrm1",
		"kcp-system",
	) // this one should be installed in the SKR, because it's not there
	mrmKCP2 := moduleReleaseMeta("mrm2", "kcp-system")
	mrmKCP3 := moduleReleaseMeta("mrm3", "kcp-system")

	mrmSKR2 := moduleReleaseMeta("mrm2", "kyma-system")
	mrmSKR3 := moduleReleaseMeta("mrm3", "kyma-system")
	mrmSKR4 := moduleReleaseMeta("mrm4", "kyma-system") // this one should be deleted, because it's not in the KCP

	// Create a fake client with the SKR objects
	scheme, err := v1beta2.SchemeBuilder.Build()
	require.NoError(t, err)
	skrClient := fake.NewClientBuilder().
		WithObjects(&mrmSKR2, &mrmSKR3, &mrmSKR4).
		WithScheme(scheme).
		Build()

	onSyncConcurrentlyFn := func(_ context.Context, kcpModules []v1beta2.ModuleReleaseMeta) {
		if len(kcpModules) != 3 {
			t.Errorf("Expected 3 kcp modules, got %d", len(kcpModules))
		}
		if kcpModules[0].Name != "mrm1" {
			t.Errorf("Expected module mrm1, got %s", kcpModules[0].Name)
		}
		if kcpModules[1].Name != "mrm2" {
			t.Errorf("Expected module mrm2, got %s", kcpModules[1].Name)
		}
		if kcpModules[2].Name != "mrm3" {
			t.Errorf("Expected module mrm3, got %s", kcpModules[2].Name)
		}
	}

	onDeleteConcurrentlyFn := func(_ context.Context, runtimeModules []v1beta2.ModuleReleaseMeta) {
		if len(runtimeModules) != 1 {
			t.Errorf("Expected 1 runtime module, got %d", len(runtimeModules))
		}
		if runtimeModules[0].Name != "mrm4" {
			t.Errorf("Expected module mrm4, got %s", runtimeModules[0].Name)
		}
	}

	syncWorkerFactoryFn := func(kcpClient, skrClient client.Client, settings *Settings) moduleReleaseMetaSyncWorker {
		return &fakeModuleReleaseMetaSyncWorker{
			namespace:            settings.Namespace,
			onSyncConcurrently:   onSyncConcurrentlyFn,
			onDeleteConcurrently: onDeleteConcurrentlyFn,
		}
	}

	subject := moduleReleaseMetaSyncer{
		skrClient:           skrClient,
		settings:            getSettings(),
		syncWorkerFactoryFn: syncWorkerFactoryFn,
	}

	// when
	err = subject.SyncToSKR(t.Context(), []v1beta2.ModuleReleaseMeta{mrmKCP1, mrmKCP2, mrmKCP3})

	// then
	assert.NoError(t, err)
}

// TestSyncer_SyncToSKR_nilList tests the case when the list of KCP modules is nil.
func TestModuleReleaseMetaSyncer_SyncToSKR_nilList(t *testing.T) {
	// given
	mtSKR2 := moduleReleaseMeta("mrm2", "kyma-system") // should be deleted, because it's not in the KCP
	mtSKR3 := moduleReleaseMeta("mrm3", "kyma-system") // should be deleted, because it's not in the KCP
	mtSKR4 := moduleReleaseMeta("mrm4", "kyma-system") // should be deleted, because it's not in the KCP

	// Create a fake client with the SKR modules
	scheme, err := v1beta2.SchemeBuilder.Build()
	require.NoError(t, err)
	skrClient := fake.NewClientBuilder().
		WithObjects(&mtSKR2, &mtSKR3, &mtSKR4).
		WithScheme(scheme).
		Build()

	// onSyncConcurrentlyFn "pretends" to be the moduleReleaseMetaConcurrentWorker.SyncConcurrently
	onSyncConcurrentlyFn := func(_ context.Context, kcpModules []v1beta2.ModuleReleaseMeta) {
		if kcpModules != nil {
			t.Errorf("Expected nil kcp modules, got %v", kcpModules)
		}
	}

	// onDeleteConcurrentlyFn "pretends" to be the moduleReleaseMetaConcurrentWorker.DeleteConcurrently
	onDeleteConcurrentlyFn := func(_ context.Context, runtimeModules []v1beta2.ModuleReleaseMeta) {
		if len(runtimeModules) != 3 {
			t.Errorf("Expected 3 runtime module, got %d", len(runtimeModules))
		}
		if runtimeModules[0].Name != "mrm2" {
			t.Errorf("Expected module mt2, got %s", runtimeModules[0].Name)
		}
		if runtimeModules[1].Name != "mrm3" {
			t.Errorf("Expected module mt2, got %s", runtimeModules[1].Name)
		}
		if runtimeModules[2].Name != "mrm4" {
			t.Errorf("Expected module mt2, got %s", runtimeModules[2].Name)
		}
	}

	syncWorkerFactoryFn := func(kcpClient, skrClient client.Client, settings *Settings) moduleReleaseMetaSyncWorker {
		return &fakeModuleReleaseMetaSyncWorker{
			namespace:            settings.Namespace,
			onSyncConcurrently:   onSyncConcurrentlyFn,
			onDeleteConcurrently: onDeleteConcurrentlyFn,
		}
	}

	subject := moduleReleaseMetaSyncer{
		skrClient:           skrClient,
		settings:            getSettings(),
		syncWorkerFactoryFn: syncWorkerFactoryFn,
	}

	// when
	var nilModuleReleaseMetaList []v1beta2.ModuleReleaseMeta = nil
	err = subject.SyncToSKR(t.Context(), nilModuleReleaseMetaList)

	// then
	assert.NoError(t, err)
}

func moduleReleaseMeta(name, namespace string) v1beta2.ModuleReleaseMeta {
	return v1beta2.ModuleReleaseMeta{
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

// Implements the syncWorker interface.
type fakeModuleReleaseMetaSyncWorker struct {
	namespace            string
	onSyncConcurrently   func(ctx context.Context, kcpModules []v1beta2.ModuleReleaseMeta)
	onDeleteConcurrently func(ctx context.Context, runtimeModules []v1beta2.ModuleReleaseMeta)
}

func (f *fakeModuleReleaseMetaSyncWorker) SyncConcurrently(
	ctx context.Context,
	kcpModules []v1beta2.ModuleReleaseMeta,
) error {
	f.onSyncConcurrently(ctx, kcpModules)

	// Simulate namespace switch on modules in kcpModules list that happens in
	// moduleReleaseMetaConcurrentWorker.SyncConcurrently
	// This is necessary for proper diff calculation later in the process.
	for i := range kcpModules {
		prepareModuleReleaseMetaForSSA(&kcpModules[i], f.namespace)
	}

	return nil
}

func (f *fakeModuleReleaseMetaSyncWorker) DeleteConcurrently(
	ctx context.Context,
	runtimeModules []v1beta2.ModuleReleaseMeta,
) error {
	f.onDeleteConcurrently(ctx, runtimeModules)
	return nil
}
