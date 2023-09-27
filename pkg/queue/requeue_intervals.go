package queue

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type RequeueIntervals struct {
	Success time.Duration
	Busy    time.Duration
	Error   time.Duration
}

func DetermineRequeueInterval(state v1beta2.State, intervals RequeueIntervals) time.Duration {
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
