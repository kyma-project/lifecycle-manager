package queue

import (
	"math/rand"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type RequeueIntervals struct {
	Success time.Duration
	Busy    time.Duration
	Warning time.Duration
	Error   time.Duration
	Jitter  *RequeueJitter
}

func DetermineRequeueInterval(state shared.State, intervals RequeueIntervals) time.Duration {
	var interval time.Duration
	switch state {
	case shared.StateError:
		interval = intervals.Error
	case shared.StateDeleting:
		interval = intervals.Busy
	case shared.StateProcessing:
		interval = intervals.Busy
	case shared.StateWarning:
		interval = intervals.Warning
	case shared.StateReady:
		interval = intervals.Success
	default:
		interval = intervals.Success
	}

	if intervals.Jitter != nil {
		return intervals.Jitter.Apply(interval)
	}
	return interval
}

type RequeueJitter struct {
	Enabled           bool
	StartedAt         time.Time
	DisableAfter      time.Duration
	JitterProbability float64
	JitterPercentage  float64
}

func (j *RequeueJitter) Apply(interval time.Duration) time.Duration {
	if !j.Enabled {
		return interval
	}

	if time.Since(j.StartedAt) > j.DisableAfter {
		j.Enabled = false
		return interval
	}

	//nolint:gosec // No simple way to generate a float in crypto/rand lib and this is not a security-sensitive context
	if rand.Float64() < j.JitterProbability {
		//nolint:gosec,gomnd // gosec: Same as above and gomnd: 2 is part of the formula
		jitter := rand.Float64()*(2*j.JitterPercentage) - j.JitterPercentage
		return time.Duration(float64(interval) * (1 + jitter))
	}

	return interval
}

type RequeueType string

const (
	IntendedRequeue   RequeueType = "intended"
	UnexpectedRequeue RequeueType = "unexpected"
)
