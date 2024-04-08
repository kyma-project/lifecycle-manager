package remote

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ClientLookup struct {
	kcp   Client
	cache *ClientCache
}

func NewClientLookup(kcp Client, cache *ClientCache) *ClientLookup {
	return &ClientLookup{kcp: kcp, cache: cache}
}

func (l *ClientLookup) Lookup(ctx context.Context, key client.ObjectKey) (Client, error) {
	remoteClient := l.cache.Get(key)
	if remoteClient != nil {
		return remoteClient, nil
	}

	clusterClient := ClusterClient{
		DefaultClient: l.kcp,
		Logger:        logf.FromContext(ctx),
	}

	restConfig, err := clusterClient.GetRestConfigFromSecret(ctx, key.Name, key.Namespace)
	if err != nil {
		return nil, err
	}

	restConfig.QPS = l.kcp.Config().QPS
	restConfig.Burst = l.kcp.Config().Burst

	clnt, err := client.New(restConfig, client.Options{Scheme: l.kcp.Scheme()})
	if err != nil {
		return nil, fmt.Errorf("failed to create lookup client: %w", err)
	}

	skr := NewClientWithConfig(clnt, restConfig)

	l.cache.Add(key, skr)

	return skr, nil
}
