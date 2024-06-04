package v2

import (
	"context"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	EventRecorderDefault = "declarative.kyma-project.io/events"
)

// TODO: os.TempDir()

func DefaultOptions() *Options {
	return (&Options{}).Apply(
		WithSingletonClientCache(NewMemoryClientCache()),
	)
}

type Options struct {
	record.EventRecorder
	Config *rest.Config
	client.Client
	TargetCluster ClusterFn

	SpecResolver
	ClientCache
	ClientCacheKeyFn
}

type Option interface {
	Apply(options *Options)
}

func (o *Options) Apply(options ...Option) *Options {
	for i := range options {
		options[i].Apply(o)
	}
	return o
}

type WithManagerOption struct {
	manager.Manager
}

func WithManager(mgr manager.Manager) WithManagerOption {
	return WithManagerOption{Manager: mgr}
}

func (o WithManagerOption) Apply(options *Options) {
	options.EventRecorder = o.GetEventRecorderFor(EventRecorderDefault)
	options.Config = o.GetConfig()
	options.Client = o.GetClient()
}

func WithSpecResolver(resolver SpecResolver) SpecResolverOption {
	return SpecResolverOption{resolver}
}

type SpecResolverOption struct {
	SpecResolver
}

func (o SpecResolverOption) Apply(options *Options) {
	options.SpecResolver = o
}

type WithSingletonClientCacheOption struct {
	ClientCache
}

func WithSingletonClientCache(cache ClientCache) WithSingletonClientCacheOption {
	return WithSingletonClientCacheOption{ClientCache: cache}
}

func (o WithSingletonClientCacheOption) Apply(options *Options) {
	options.ClientCache = o
}

type ClusterFn func(context.Context, Object) (*ClusterInfo, error)

func WithRemoteTargetCluster(configFn ClusterFn) WithRemoteTargetClusterOption {
	return WithRemoteTargetClusterOption{ClusterFn: configFn}
}

type WithRemoteTargetClusterOption struct {
	ClusterFn
}

func (o WithRemoteTargetClusterOption) Apply(options *Options) {
	options.TargetCluster = o.ClusterFn
}

type ClientCacheKeyFn func(ctx context.Context, obj Object) (string, bool)

type WithClientCacheKeyOption struct {
	ClientCacheKeyFn
}

func (o WithClientCacheKeyOption) Apply(options *Options) {
	options.ClientCacheKeyFn = o.ClientCacheKeyFn
}
