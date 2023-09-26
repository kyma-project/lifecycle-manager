package controllers

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewCacheOptions() cache.Options {
	cacheLabelSelector := labels.SelectorFromSet(
		labels.Set{v1beta2.ManagedBy: v1beta2.OperatorName},
	)
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {Label: cacheLabelSelector},
			&corev1.Service{}: {Label: labels.SelectorFromSet(labels.Set{
				"app": "istio-ingressgateway",
			})},
		},
	}
}

type RequeueIntervals struct {
	Success time.Duration
	Busy    time.Duration
	Error   time.Duration
}

func determineRequeueInterval(state v1beta2.State, intervals RequeueIntervals) time.Duration {
	switch state {
	case v1beta2.StateError:
		return intervals.Error
	case v1beta2.StateDeleting:
		fallthrough
	case v1beta2.StateProcessing:
		return intervals.Busy
	case v1beta2.StateReady:
		fallthrough
	case v1beta2.StateWarning:
		fallthrough
	default:
		return intervals.Success
	}
}
