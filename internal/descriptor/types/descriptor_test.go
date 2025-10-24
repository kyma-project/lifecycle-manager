package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ocm.software/ocm/api/ocm/compdesc"
	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
)

const (
	testComponentName    = "kyma-project.io/test-component"
	testComponentVersion = "1.2.3"
)

func TestDeserialize(t *testing.T) {
	compdesc.RegisterScheme(&compdescv2.DescriptorVersion{})

	t.Run("Deserialize valid component descriptor should succeed", func(t *testing.T) {
		cd := compdesc.New(testComponentName, testComponentVersion)
		cdBytes, err := compdesc.Encode(cd)
		require.NoError(t, err)

		compDesc, err := types.Deserialize(cdBytes)
		require.NoError(t, err)
		assert.Equal(t, testComponentName, compDesc.Name)
		assert.Equal(t, testComponentVersion, compDesc.Version)
	})

	t.Run("Deserialize invalid component descriptor should return an error", func(t *testing.T) {
		invalidCompDescBytes := []byte("invalid component descriptor content")

		_, err := types.Deserialize(invalidCompDescBytes)
		require.Error(t, err)
		assert.ErrorIs(t, err, types.ErrDecode)
	})
}
