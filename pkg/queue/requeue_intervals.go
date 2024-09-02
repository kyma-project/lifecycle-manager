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
	jitterProbability float64
	jitterPercentage  float64
	randFunc          func() float64
}

func NewRequeueJitter(jitterProbability, jitterPercentage float64) *RequeueJitter {
	return &RequeueJitter{
		jitterProbability: jitterProbability,
		jitterPercentage:  jitterPercentage,
		randFunc:          rand.Float64,
	}
}

func (j *RequeueJitter) Apply(interval time.Duration) time.Duration {
	//nolint:gosec // No simple way to generate a float in crypto/rand lib and this is not a security-sensitive context
	if j.randFunc() <= j.jitterProbability {
		//nolint:gosec,gomnd // gosec: Same as above and gomnd: 2 is part of the formula
		jitter := j.randFunc()*(2*j.jitterPercentage) - j.jitterPercentage
		return time.Duration(float64(interval) * (1 + jitter))
	}

	return interval
}

type RequeueType string

const (
	IntendedRequeue   RequeueType = "intended"
	UnexpectedRequeue RequeueType = "unexpected"
)
