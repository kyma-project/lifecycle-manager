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
)

const (
	EventRecorderDefault    = "declarative.kyma-project.io/events"
	DefaultInMemoryParseTTL = 24 * time.Hour
)

func DefaultOptions() *Options {
	return (&Options{}).Apply(
		WithPostRenderTransform(
			ManagedByDeclarativeV2,
			watchedByOwnedBy,
			KymaComponentTransform,
			DisclaimerTransform,
		),
		WithSingletonClientCache(NewMemoryClientCache()),
		WithManifestCache(os.TempDir()),
		WithManifestParser(NewInMemoryCachedManifestParser(DefaultInMemoryParseTTL)),
		WithModuleCRDeletionCheck(NewDefaultDeletionCheck()),
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
	ManifestParser
	ManifestCache
	CustomReadyCheck ReadyCheck

	PostRenderTransforms []ObjectTransform

	PostRuns   []PostRun
	PreDeletes []PreDelete

	DeletionCheck ModuleCRDeletionCheck

	DeletePrerequisites bool
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

func WithSpecResolver(resolver SpecResolver) SpecResolverOption {
	return SpecResolverOption{resolver}
}

type SpecResolverOption struct {
	SpecResolver
}

func (o SpecResolverOption) Apply(options *Options) {
	options.SpecResolver = o
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

func WithModuleCRDeletionCheck(deletionCheckFn ModuleCRDeletionCheck) WithModuleCRDeletionCheckOption {
	return WithModuleCRDeletionCheckOption{ModuleCRDeletionCheck: deletionCheckFn}
}

type WithModuleCRDeletionCheckOption struct {
	ModuleCRDeletionCheck
}

func (o WithModuleCRDeletionCheckOption) Apply(options *Options) {
	options.DeletionCheck = o
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

type WithCustomReadyCheckOption struct {
	ReadyCheck
}

func WithCustomReadyCheck(check ReadyCheck) WithCustomReadyCheckOption {
	return WithCustomReadyCheckOption{ReadyCheck: check}
}

func (o WithCustomReadyCheckOption) Apply(options *Options) {
	options.CustomReadyCheck = o
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
