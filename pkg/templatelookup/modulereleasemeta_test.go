package templatelookup_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_GetChannelVersionForModule_WhenEmptyChannels(t *testing.T) {
	moduleReleaseMeta := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Channels: nil,
		},
	}
	_, err := templatelookup.GetChannelVersionForModule(moduleReleaseMeta, "test")

	require.ErrorContains(t, err, "no channels found for module")
}

func Test_GetChannelVersionForModule_WhenChannelFound(t *testing.T) {
	moduleReleaseMeta := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Channels: []v1beta2.ChannelVersionAssignment{
				{
					Channel: "regular",
					Version: "1.0.0",
				},
			},
		},
	}
	version, err := templatelookup.GetChannelVersionForModule(moduleReleaseMeta, "regular")

	require.NoError(t, err)
	require.Equal(t, "1.0.0", version)
}

func Test_GetChannelVersionForModule_WhenChannelNotFound(t *testing.T) {
	moduleReleaseMeta := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Channels: []v1beta2.ChannelVersionAssignment{
				{
					Channel: "regular",
					Version: "1.0.0",
				},
			},
		},
	}
	_, err := templatelookup.GetChannelVersionForModule(moduleReleaseMeta, "fast")

	require.ErrorContains(t, err, "no versions found for channel")
}
