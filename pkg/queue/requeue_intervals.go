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
		fallthrough
	case shared.StateUnmanaged:
		fallthrough
	default:
		interval = intervals.Success
	}

	if intervals.Jitter != nil {
		return intervals.Jitter.Apply(interval)
	}
	return interval
}

type RequeueJitter struct {
	JitterProbability float64
	JitterPercentage  float64
	RandFunc          func() float64
}

func NewRequeueJitter(jitterProbability, jitterPercentage float64) *RequeueJitter {
	return &RequeueJitter{
		JitterProbability: jitterProbability,
		JitterPercentage:  jitterPercentage,
		RandFunc:          rand.Float64,
	}
}

func (j *RequeueJitter) Apply(interval time.Duration) time.Duration {
	if j.RandFunc() <= j.JitterProbability {
		jitter := j.RandFunc()*(2*j.JitterPercentage) - j.JitterPercentage //nolint:mnd // 2 is part of the formula
		return time.Duration(float64(interval) * (1 + jitter))
	}

	return interval
}

type RequeueType string

const (
	IntendedRequeue   RequeueType = "intended"
	UnexpectedRequeue RequeueType = "unexpected"
)
