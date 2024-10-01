package v2

import (
	"context"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	EventRecorderDefault    = "declarative.kyma-project.io/events"
	DefaultInMemoryParseTTL = 24 * time.Hour
)

func DefaultOptions() *Options {
	return (&Options{}).Apply(
		WithPostRenderTransform(
			WatchedByManagedByOwnedBy,
			KymaComponentTransform,
			DisclaimerTransform,
		),
		WithSingletonClientCache(NewMemoryClientCache()),
		WithManifestCache(os.TempDir()),
		WithManifestParser(NewInMemoryCachedManifestParser(DefaultInMemoryParseTTL)),
		WithCustomResourceLabels{
			shared.ManagedBy: shared.ManagedByLabelValue,
		},
	)
}

type Options struct {
	record.EventRecorder
	Config *rest.Config
	client.Client
	TargetCluster ClusterFn

	ClientCache
	ClientCacheKeyFn
	ManifestParser
	ManifestCache
	CustomStateCheck StateCheck

	PostRenderTransforms []ObjectTransform

	PostRuns   []PostRun
	PreDeletes []PreDelete
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

type WithCustomResourceLabels k8slabels.Set

func (o WithCustomResourceLabels) Apply(options *Options) {
	labelTransform := func(ctx context.Context, object Object, resources []*unstructured.Unstructured) error {
		for _, targetResource := range resources {
			lbls := targetResource.GetLabels()
			if lbls == nil {
				lbls = k8slabels.Set{}
			}
			for s := range o {
				lbls[s] = o[s]
			}
			targetResource.SetLabels(lbls)
		}
		return nil
	}
	options.PostRenderTransforms = append(options.PostRenderTransforms, labelTransform)
}

type ObjectTransform = func(context.Context, Object, []*unstructured.Unstructured) error

func WithPostRenderTransform(transforms ...ObjectTransform) PostRenderTransformOption {
	return PostRenderTransformOption{transforms}
}

type PostRenderTransformOption struct {
	ObjectTransforms []ObjectTransform
}

func (o PostRenderTransformOption) Apply(options *Options) {
	options.PostRenderTransforms = append(options.PostRenderTransforms, o.ObjectTransforms...)
}

// Hook defines a Hook into the declarative reconciliation
// skr is the runtime cluster
// kcp is the control-plane cluster
// obj is guaranteed to be the reconciled object and also to always preside in kcp.
type Hook func(ctx context.Context, skr Client, kcp client.Client, obj Object) error

// WARNING: DO NOT USE THESE HOOKS IF YOU DO NOT KNOW THE RECONCILIATION LIFECYCLE OF THE DECLARATIVE API.
// IT CAN BREAK YOUR RECONCILIATION AND IF YOU ADJUST THE OBJECT, ALSO LEAD TO
// INVALID STATES.
type (
	// PostRun is executed after every successful render+reconciliation of the manifest.
	PostRun Hook
	// PreDelete is executed before any deletion of resources calculated from the status.
	PreDelete Hook
)

// WithPostRun applies PostRun.
type WithPostRun []PostRun

func (o WithPostRun) Apply(options *Options) {
	options.PostRuns = append(options.PostRuns, o...)
}

// WithPreDelete applies PreDelete.
type WithPreDelete []PreDelete

func (o WithPreDelete) Apply(options *Options) {
	options.PreDeletes = append(options.PreDeletes, o...)
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

type ManifestCache string

type WithManifestCache ManifestCache

func (o WithManifestCache) Apply(options *Options) {
	options.ManifestCache = ManifestCache(o)
}

func WithManifestParser(parser ManifestParser) WithManifestParserOption {
	return WithManifestParserOption{ManifestParser: parser}
}

type WithManifestParserOption struct {
	ManifestParser
}

func (o WithManifestParserOption) Apply(options *Options) {
	options.ManifestParser = o.ManifestParser
}

type WithCustomStateCheckOption struct {
	StateCheck
}

func WithCustomStateCheck(check StateCheck) WithCustomStateCheckOption {
	return WithCustomStateCheckOption{StateCheck: check}
}

func (o WithCustomStateCheckOption) Apply(options *Options) {
	options.CustomStateCheck = o
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
