package v1beta2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func Test_ModuleReleaseMeta_IsMandatory(t *testing.T) {
	tests := []struct {
		name string
		mrm  v1beta2.ModuleReleaseMeta
		want bool
	}{
		{
			name: "When Mandatory is set, Then IsMandatory returns true",
			mrm: v1beta2.ModuleReleaseMeta{
				Spec: v1beta2.ModuleReleaseMetaSpec{
					Mandatory: &v1beta2.Mandatory{Version: "1.0.0"},
				},
			},
			want: true,
		},
		{
			name: "When Mandatory is nil, Then IsMandatory returns false",
			mrm: v1beta2.ModuleReleaseMeta{
				Spec: v1beta2.ModuleReleaseMetaSpec{
					Channels: []v1beta2.ChannelVersionAssignment{{Channel: "regular", Version: "1.0.0"}},
				},
			},
			want: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, testCase.mrm.IsMandatory())
		})
	}
}
