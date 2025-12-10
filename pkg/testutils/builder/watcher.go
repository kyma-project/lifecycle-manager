package builder

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type WatcherBuilder struct {
	watcher *v1beta2.Watcher
}

func NewWatcherBuilder() *WatcherBuilder {
	builder := &WatcherBuilder{
		watcher: &v1beta2.Watcher{},
	}

	namespace := random.Name()

	return builder.
		WithName(random.Name()).
		WithNamespace(namespace).
		WithManager(random.Name()).
		WithServiceInfoName(random.Name()).
		WithServiceInfoNamespace(namespace).
		WithServiceInfoPort(random.Port()).
		WithAPIVersion("operator.kyma-project.io/v1beta2").
		WithKind("Watcher").
		WithUID(uuid.NewUUID())
}

func (builder *WatcherBuilder) Build() *v1beta2.Watcher {
	return builder.watcher
}

func (builder *WatcherBuilder) WithName(name string) *WatcherBuilder {
	builder.watcher.SetName(name)
	return builder
}

func (builder *WatcherBuilder) WithNamespace(namespace string) *WatcherBuilder {
	builder.watcher.SetNamespace(namespace)
	return builder
}

func (builder *WatcherBuilder) WithManager(manager string) *WatcherBuilder {
	builder.watcher.Spec.Manager = manager
	return builder
}

func (builder *WatcherBuilder) WithServiceInfoName(name string) *WatcherBuilder {
	builder.watcher.Spec.ServiceInfo.Name = name
	return builder
}

func (builder *WatcherBuilder) WithServiceInfoNamespace(namespace string) *WatcherBuilder {
	builder.watcher.Spec.ServiceInfo.Namespace = namespace
	return builder
}

func (builder *WatcherBuilder) WithServiceInfoPort(port int64) *WatcherBuilder {
	builder.watcher.Spec.ServiceInfo.Port = port
	return builder
}

func (builder *WatcherBuilder) WithAPIVersion(apiVersion string) *WatcherBuilder {
	builder.watcher.APIVersion = apiVersion
	return builder
}

func (builder *WatcherBuilder) WithKind(kind string) *WatcherBuilder {
	builder.watcher.Kind = kind
	return builder
}

func (builder *WatcherBuilder) WithUID(uid types.UID) *WatcherBuilder {
	builder.watcher.UID = uid
	return builder
}
