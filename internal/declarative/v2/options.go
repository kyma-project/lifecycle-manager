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
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

const (
	EventRecorderDefault    = "declarative.kyma-project.io/events"
	DefaultInMemoryParseTTL = 24 * time.Hour
)

func DefaultOptions() *Options {
	return (&Options{}).Apply(
		WithPostRenderTransform(
			ManagedByOwnedBy,
			KymaComponentTransform,
			DisclaimerTransform,
			DockerImageLocalizationTransform,
		),
		WithManifestCache(os.TempDir()),
		WithManifestParser(NewInMemoryManifestCache(DefaultInMemoryParseTTL)),
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
	ManifestParser
	ManifestCache
	CustomStateCheck StateCheck

	PostRenderTransforms []ObjectTransform
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

type ClusterFn func(context.Context, Object) (*skrclient.ClusterInfo, error)

func WithRemoteTargetCluster(configFn ClusterFn) WithRemoteTargetClusterOption {
	return WithRemoteTargetClusterOption{ClusterFn: configFn}
}

type WithRemoteTargetClusterOption struct {
	ClusterFn
}

func (o WithRemoteTargetClusterOption) Apply(options *Options) {
	options.TargetCluster = o.ClusterFn
}
