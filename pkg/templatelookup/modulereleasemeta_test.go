package templatelookup_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

func Test_GetChannelVersionForModule_WhenEmptyChannels(t *testing.T) {
	moduleReleaseMeta := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Channels: nil,
		},
	}
	_, err := templatelookup.GetChannelVersionForModule(moduleReleaseMeta, "test")

	require.ErrorIs(t, err, templatelookup.ErrNoChannelsFound)
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

	require.ErrorIs(t, err, templatelookup.ErrChannelNotFound)
}

func Test_GetMandatoryVersionForModule_WhenMandatoryFound(t *testing.T) {
	moduleReleaseMeta := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Mandatory: &v1beta2.Mandatory{
				Version: "1.0.0",
			},
		},
	}
	version, err := templatelookup.GetMandatoryVersionForModule(moduleReleaseMeta)

	require.NoError(t, err)
	require.Equal(t, "1.0.0", version)
}

func Test_GetMandatoryVersionForModule_WhenMandatoryNotFound(t *testing.T) {
	moduleReleaseMeta := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			Mandatory: nil,
		},
	}
	_, err := templatelookup.GetMandatoryVersionForModule(moduleReleaseMeta)

	require.ErrorIs(t, err, templatelookup.ErrNoMandatoryFound)
}
