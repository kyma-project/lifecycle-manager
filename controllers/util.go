package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

func NewCacheFunc() cache.NewCacheFunc {
	cacheLabelSelector := labels.SelectorFromSet(
		labels.Set{v1beta1.ManagedBy: v1beta1.OperatorName},
	)
	return cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&v1beta1.ModuleTemplate{}: {Label: cacheLabelSelector},
			&corev1.Secret{}:          {Label: cacheLabelSelector},
			&corev1.Service{}: {Label: labels.SelectorFromSet(labels.Set{
				"app": "istio-ingressgateway",
			})},
		},
	})
}
