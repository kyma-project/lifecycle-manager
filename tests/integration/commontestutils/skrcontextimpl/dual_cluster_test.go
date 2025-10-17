package skrcontextimpl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/event"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// fakeEvent implements event.Event but no-ops
// to avoid requiring a real recorder for these focused tests.
type fakeEvent struct{}

func (f fakeEvent) Normal(_ runtime.Object, _ event.Reason, _ string) {}
func (f fakeEvent) Warning(_ runtime.Object, _ event.Reason, _ error) {}

// TestDualClusterFactory_ReusesSingleEnvironment ensures that multiple Init calls
// for different Kyma names reuse a single envtest environment.
func TestDualClusterFactory_ReusesSingleEnvironment(t *testing.T) {
	scheme := runtime.NewScheme()
	factory := NewDualClusterFactory(scheme, fakeEvent{})

	// Skip test if kubebuilder assets (etcd, kube-apiserver) are not present.
	// This keeps the unit test lightweight while still allowing it to run
	// in CI where assets are provisioned (e.g. via setup-envtest).
	assetDir := os.Getenv("KUBEBUILDER_ASSETS")
	if assetDir == "" {
		// fallback to common default location
		assetDir = "/usr/local/kubebuilder/bin"
	}
	if _, err := os.Stat(filepath.Join(assetDir, "etcd")); err != nil {
		t.Skipf("kubebuilder assets not found in %s, skipping envtest-dependent test", assetDir)
	}

	kymaA := types.NamespacedName{Name: "kyma-a"}
	kymaB := types.NamespacedName{Name: "kyma-b"}

	if err := factory.Init(context.Background(), kymaA); err != nil {
		t.Fatalf("unexpected error on first init: %v", err)
	}
	firstEnv := factory.GetSkrEnv()
	if firstEnv == nil {
		t.Fatalf("expected skr env to be initialized")
	}

	if err := factory.Init(context.Background(), kymaB); err != nil {
		t.Fatalf("unexpected error on second init: %v", err)
	}

	secondEnv := factory.GetSkrEnv()
	if secondEnv != firstEnv {
		t.Fatalf("expected the same envtest environment to be reused")
	}

	// Ensure both clients can be retrieved
	if _, err := factory.Get(kymaA); err != nil {
		t.Fatalf("expected client for kymaA: %v", err)
	}
	if _, err := factory.Get(kymaB); err != nil {
		t.Fatalf("expected client for kymaB: %v", err)
	}

	if err := factory.Stop(); err != nil {
		t.Fatalf("unexpected error on stop: %v", err)
	}
}
