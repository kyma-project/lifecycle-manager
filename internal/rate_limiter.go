package internal

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
)

func ManifestRateLimiter(
	failureBaseDelay time.Duration, failureMaxDelay time.Duration,
	frequency int, burst int,
) ratelimiter.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(failureBaseDelay, failureMaxDelay),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(frequency), burst)},
	)
}
