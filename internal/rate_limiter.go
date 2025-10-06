package internal

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
)

func RateLimiter(
	failureBaseDelay time.Duration, failureMaxDelay time.Duration,
	frequency int, burst int,
) workqueue.TypedRateLimiter[ctrl.Request] {
	return workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](failureBaseDelay, failureMaxDelay),
		&workqueue.TypedBucketRateLimiter[ctrl.Request]{Limiter: rate.NewLimiter(rate.Limit(frequency), burst)},
	)
}
