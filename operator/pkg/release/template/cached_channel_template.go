package template

import (
	"context"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewCachedChannelBasedFinder(
	client client.Reader,
	component operatorv1alpha1.ComponentType,
	channel operatorv1alpha1.Channel,
	cache Cache,
) Finder {
	return &cachedChannelBasedFinder{
		channelBasedFinder: &channelBasedFinder{
			reader:    client,
			component: component,
			channel:   channel,
		},
		cache: cache,
	}
}

type cachedChannelBasedFinder struct {
	channelBasedFinder *channelBasedFinder
	cache              Cache
}

func (c *cachedChannelBasedFinder) Lookup(ctx context.Context) (*LookupResult, error) {
	v, err := c.cache.Get(GetTemplateCacheKey(c.channelBasedFinder.component.Name, c.channelBasedFinder.channel))
	if err != nil {
		return nil, err
	}
	if v == nil {
		lookupResult, err := c.channelBasedFinder.Lookup(ctx)
		if lookupResult == nil {
			return nil, err
		}
		if err := c.cache.Put(GetTemplateCacheKey(c.channelBasedFinder.component.Name, c.channelBasedFinder.channel), lookupResult); err != nil {
			return nil, err
		}
		return lookupResult, nil
	} else {
		return v, nil
	}
}
