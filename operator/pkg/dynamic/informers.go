package dynamic

import (
	"context"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func Informers(mgr manager.Manager, filter GroupFilter) (map[string]source.Source, error) {

	c, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(c, defaultResync)

	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		informerFactory.Start(ctx.Done())

		return nil
	}))
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	apiGroupList, err := clientSet.ServerGroups()
	if err != nil {
		return nil, err
	}

	var groupVersions []schema.GroupVersion
	for _, groupFromServer := range apiGroupList.Groups {
		for _, filterGroup := range filter {
			if strings.Contains(groupFromServer.Name, filterGroup) {
				gv, err := schema.ParseGroupVersion(groupFromServer.PreferredVersion.GroupVersion)
				if err != nil {
					return nil, err
				}
				groupVersions = append(groupVersions, gv)
			}
		}
	}

	var resources []v1.APIResource

	for _, groupVersion := range groupVersions {
		resFromGv, err := clientSet.ServerResourcesForGroupVersion(groupVersion.String())
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		for _, apiResource := range resFromGv.APIResources {
			if strings.HasSuffix(apiResource.Name, "status") {
				continue
			}
			apiResource.Group = groupVersion.Group
			apiResource.Version = groupVersion.Version
			resources = append(resources, apiResource)
		}
	}

	if err != nil {
		return nil, err
	}

	dynamicInformerSet := make(map[string]source.Source)

	for _, resource := range resources {
		gvr := schema.GroupVersionResource{
			Group:    resource.Group,
			Version:  resource.Version,
			Resource: resource.Name,
		}
		informer := informerFactory.ForResource(gvr).Informer()
		dynamicInformerSet[gvr.String()] = &ComponentInformer{
			Informer:             source.Informer{Informer: informer},
			GroupVersionResource: gvr,
		}
	}

	return dynamicInformerSet, nil
}
