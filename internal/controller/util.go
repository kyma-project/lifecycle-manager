package controller

import (
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func NewCacheOptions() cache.Options {
	cacheLabelSelector := k8slabels.SelectorFromSet(
		k8slabels.Set{v1beta2.ManagedBy: v1beta2.OperatorName},
	)
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&apicorev1.Secret{}: {Label: cacheLabelSelector},
			&apicorev1.Service{}: {
				Label: k8slabels.SelectorFromSet(k8slabels.Set{
					"app": "istio-ingressgateway",
				}),
			},
		},
	}
}
