package dynamic

import (
	"context"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"
)

func Informers(mgr manager.Manager) (map[string]*source.Informer, error) {
	c, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	informers := dynamicinformer.NewDynamicSharedInformerFactory(c, time.Minute*30)
	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		informers.Start(ctx.Done())
		return nil
	}))
	if err != nil {
		return nil, err
	}

	//TODO maybe replace with native REST Handling
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}
	// This fetches all resources for our component operator CRDs, might become a problem if component operators
	// create their own CRDs that we dont need to watch
	gv := schema.GroupVersion{
		Group:   labels.ComponentPrefix,
		Version: "v1alpha1",
	}
	resources, err := cs.ServerResourcesForGroupVersion(gv.String())
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	dynamicInformerSet := make(map[string]*source.Informer)
	for _, resource := range resources.APIResources {
		//TODO Verify if this filtering is really necessary or if we can somehow only listen to status changes instead of resource changes with ResourceVersionChangedPredicate
		if strings.HasSuffix(resource.Name, "status") {
			continue
		}
		gvr := gv.WithResource(resource.Name)
		dynamicInformerSet[gvr.String()] = &source.Informer{Informer: informers.ForResource(gvr).Informer()}
	}
	return dynamicInformerSet, nil
}
