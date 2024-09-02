package queue

import (
	"testing"
	"time"
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jitter := &RequeueJitter{
				jitterProbability: tt.jitterProbability,
				jitterPercentage:  tt.jitterPercentage,
				randFunc: func() float64 {
					return tt.randValue
				},
			}

			result := jitter.Apply(tt.interval)

			if result < tt.expectedMin || result > tt.expectedMax {
				t.Errorf("Apply() returned %v, expected between %v and %v", result, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}
