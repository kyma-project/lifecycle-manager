package controllers

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func NewCacheFunc() cache.NewCacheFunc {
	cacheLabelSelector := labels.SelectorFromSet(
		labels.Set{v1beta2.ManagedBy: v1beta2.OperatorName},
	)
	return cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&corev1.Secret{}: {Label: cacheLabelSelector},
			&corev1.Service{}: {Label: labels.SelectorFromSet(labels.Set{
				"app": "istio-ingressgateway",
			})},
		},
	})
}
