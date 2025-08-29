package cache_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	skrclientcache "github.com/kyma-project/lifecycle-manager/internal/service/skrclient/cache"
)

func TestService_Basic(t *testing.T) {
	svc := skrclientcache.NewService()

	require.Equal(t, 0, svc.Size(), "expected size 0")

	svc.AddClient("a", nil)
	require.Equal(t, 1, svc.Size(), "expected size 1 after add")

	require.Nil(t, svc.GetClient("a"), "expected nil client value")

	svc.AddClient("b", nil)
	require.Equal(t, 2, svc.Size(), "expected size 2 after second add")

	svc.DeleteClient("a")
	require.Equal(t, 1, svc.Size(), "expected size 1 after delete")

	require.Nil(t, svc.GetClient("a"), "expected nil for deleted key")
}
