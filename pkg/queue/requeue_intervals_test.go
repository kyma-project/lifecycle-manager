package queue_test

import (
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

func TestRequeueJitter_Apply(t *testing.T) {
	tests := []struct {
		name              string
		jitterProbability float64
		jitterPercentage  float64
		randValue         float64
		interval          time.Duration
		expectedMin       time.Duration
		expectedMax       time.Duration
	}{
		{
			name:              "No jitter applied due to low probability",
			jitterProbability: 0.0,
			jitterPercentage:  0.5,
			randValue:         0.4,
			interval:          100 * time.Millisecond,
			expectedMin:       100 * time.Millisecond,
			expectedMax:       100 * time.Millisecond,
		},
		{
			name:              "Jitter applied with max positive jitter",
			jitterProbability: 1.0,
			jitterPercentage:  0.5,
			randValue:         1.0,
			interval:          100 * time.Millisecond,
			expectedMin:       100 * time.Millisecond,
			expectedMax:       150 * time.Millisecond,
		},
		{
			name:              "Jitter applied with max negative jitter",
			jitterProbability: 1.0,
			jitterPercentage:  0.5,
			randValue:         0.0,
			interval:          100 * time.Millisecond,
			expectedMin:       50 * time.Millisecond,
			expectedMax:       100 * time.Millisecond,
		},
		{
			name:              "No jitter due to zero jitter percentage",
			jitterProbability: 1.0,
			jitterPercentage:  0.0,
			randValue:         0.5,
			interval:          100 * time.Millisecond,
			expectedMin:       100 * time.Millisecond,
			expectedMax:       100 * time.Millisecond,
		},
		{
			name:              "Jitter applied with mid-range jitter",
			jitterProbability: 1.0,
			jitterPercentage:  0.5,
			randValue:         0.5,
			interval:          100 * time.Millisecond,
			expectedMin:       75 * time.Millisecond,
			expectedMax:       125 * time.Millisecond,
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			jitter := &queue.RequeueJitter{
				JitterProbability: testcase.jitterProbability,
				JitterPercentage:  testcase.jitterPercentage,
				RandFunc: func() float64 {
					return testcase.randValue
				},
			}

			result := jitter.Apply(testcase.interval)

			if result < testcase.expectedMin || result > testcase.expectedMax {
				t.Errorf(
					"Apply() returned %v, expected between %v and %v",
					result,
					testcase.expectedMin,
					testcase.expectedMax,
				)
			}
		})
	}
}
