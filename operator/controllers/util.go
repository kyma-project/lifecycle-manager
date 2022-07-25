package controllers

import (
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func NewCacheFunc() cache.NewCacheFunc {
	cacheLabelSelector := labels.SelectorFromSet(
		labels.Set{v1alpha1.ManagedBy: v1alpha1.OperatorName},
	)
	return cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&v1alpha1.ModuleTemplate{}: {Label: cacheLabelSelector},
			&corev1.Secret{}:           {Label: cacheLabelSelector},
		},
	})
}
