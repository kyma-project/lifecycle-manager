package manifestclient_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
)

func Test_hasStatusDiff(t *testing.T) {
	type args struct {
		first  shared.Status
		second shared.Status
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Different Status",
			args: args{
				first: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.Now(),
					},
				},
				second: shared.Status{
					State: shared.StateProcessing,
					LastOperation: shared.LastOperation{
						Operation:      "installing resources",
						LastUpdateTime: apimetav1.Now(),
					},
				},
			},
			want: true,
		},
		{
			name: "Same Status",
			args: args{
				first: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.Now(),
					},
				},
				second: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.NewTime(time.Now().Add(time.Hour)),
					},
				},
			},
			want: false,
		},
		{
			name: "Empty Status",
			args: args{
				first: shared.Status{},
				second: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.NewTime(time.Now().Add(time.Hour)),
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equalf(t, tt.want, manifestclient.HasStatusDiff(tt.args.first, tt.args.second),
				"hasStatusDiff(%v, %v)",
				tt.args.first, tt.args.second)
		})
	}
}
