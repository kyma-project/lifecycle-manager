package remote

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type ClientLookup struct {
	kcp   Client
	cache *ClientCache

	strategy shared.SyncStrategy
}

func NewClientLookup(kcp Client, cache *ClientCache, strategy shared.SyncStrategy) *ClientLookup {
	return &ClientLookup{kcp: kcp, cache: cache, strategy: strategy}
}

func (l *ClientLookup) Lookup(ctx context.Context, key client.ObjectKey) (Client, error) {
	remoteClient := l.cache.Get(key)
	if remoteClient != nil {
		return remoteClient, nil
	}

	cfg, err := l.restConfigFromStrategy(ctx, key)
	if err != nil {
		return nil, err
	}

	clnt, err := client.New(cfg, client.Options{Scheme: l.kcp.Scheme()})
	if err != nil {
		return nil, fmt.Errorf("failed to create lookup client: %w", err)
	}

	skr := NewClientWithConfig(clnt, cfg)

	l.cache.Set(key, skr)

	return skr, nil
}

func (l *ClientLookup) restConfigFromStrategy(ctx context.Context, key client.ObjectKey) (*rest.Config, error) {
	var err error
	var restConfig *rest.Config

	clusterClient := ClusterClient{
		DefaultClient: l.kcp,
		Logger:        logf.FromContext(ctx),
	}
	switch l.strategy {
	case shared.SyncStrategyLocalClient:
		if LocalClient != nil {
			restConfig = LocalClient()
		} else {
			restConfig = l.kcp.Config()
		}
	case shared.SyncStrategyLocalSecret:
		fallthrough
	default:
		restConfig, err = clusterClient.GetRestConfigFromSecret(ctx, key.Name, key.Namespace)
	}

	if err != nil {
		return nil, err
	}

	// Overrides the default rate-limiting as we want unified flow control settings in KCP and SKR clusters.
	restConfig.QPS = l.kcp.Config().QPS
	restConfig.Burst = l.kcp.Config().Burst

	return restConfig, err
}
