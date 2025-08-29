package cache_test

import (
	"testing"

	skrclientcache "github.com/kyma-project/lifecycle-manager/internal/service/manifest/skrclient/cache"
)

func TestService_Basic(t *testing.T) {
	svc := skrclientcache.NewService()

	if got := svc.Size(); got != 0 {
		t.Fatalf("expected size 0, got %d", got)
	}

	svc.AddClient("a", nil)
	if got := svc.Size(); got != 1 {
		t.Fatalf("expected size 1 after add, got %d", got)
	}

	if val := svc.GetClient("a"); val != nil {
		t.Fatalf("expected nil client value, got %#v", val)
	}

	svc.AddClient("b", nil)
	if got := svc.Size(); got != 2 {
		t.Fatalf("expected size 2 after second add, got %d", got)
	}

	svc.DeleteClient("a")
	if got := svc.Size(); got != 1 {
		t.Fatalf("expected size 1 after delete, got %d", got)
	}

	if val := svc.GetClient("a"); val != nil {
		t.Fatalf("expected nil for deleted key, got %#v", val)
	}
}
