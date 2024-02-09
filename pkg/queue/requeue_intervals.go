package queue

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type RequeueIntervals struct {
	Success time.Duration
	Busy    time.Duration
	Warning time.Duration
	Error   time.Duration
}

func DetermineRequeueInterval(state shared.State, intervals RequeueIntervals) time.Duration {
	switch state {
	case shared.StateError:
		return intervals.Error
	case shared.StateDeleting:
		fallthrough
	case shared.StateProcessing:
		return intervals.Busy
	case shared.StateReady:
		fallthrough
	case shared.StateWarning:
		return intervals.Warning
	default:
		return intervals.Success
	}
}

type RequeueType string

const (
	IntendedRequeue   RequeueType = "intended"
	UnexpectedRequeue RequeueType = "unexpected"
)
