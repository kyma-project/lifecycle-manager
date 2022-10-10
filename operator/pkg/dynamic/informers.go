package dynamic

import (
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	defaultResync = 30 * time.Minute
)

type ComponentInformer struct {
	schema.GroupVersionResource
	source.Informer
}

func (ci *ComponentInformer) String() string {
	return ci.GroupVersionResource.String()
}

type GroupFilter []string

func GetDynamicInformerSources(resources []v1.APIResource, mgr manager.Manager) (map[string]source.Source, error) {
	dynamicInformerSet := make(map[string]source.Source)
	clnt, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}
	factory := dynamicinformer.NewDynamicSharedInformerFactory(clnt, defaultResync)
	for _, resource := range resources {
		groupVersionResource := schema.GroupVersionResource{
			Group:    resource.Group,
			Version:  resource.Version,
			Resource: resource.Name,
		}
		informer := factory.ForResource(groupVersionResource).Informer()
		dynamicInformerSet[groupVersionResource.String()] = &ComponentInformer{
			Informer:             source.Informer{Informer: informer},
			GroupVersionResource: groupVersionResource,
		}
	}
	return dynamicInformerSet, nil
}
