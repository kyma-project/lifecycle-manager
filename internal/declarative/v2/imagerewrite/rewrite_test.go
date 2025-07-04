package imagerewrite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/declarative/v2/imagerewrite"
)

func TestTargetImage(t *testing.T) {
	t.Parallel()

	t.Run("ValidImageReference", func(t *testing.T) {
		t.Parallel()
		// when
		targetImages, err := imagerewrite.AsTargetImages([]string{"example.com/myapp/myimage:1.2.3", "example.com:5000/myapp/myimage:3.2.1", "localhost:5111/myimage:5.4.3"})

		// then
		require.NoError(t, err)
		assert.Equal(t, "example.com/myapp", targetImages[0].HostAndPath)
		assert.Equal(t, "myimage:1.2.3", targetImages[0].NameAndTag)

		assert.Equal(t, "example.com:5000/myapp", targetImages[1].HostAndPath)
		assert.Equal(t, "myimage:3.2.1", targetImages[1].NameAndTag)

		assert.Equal(t, "localhost:5111", targetImages[2].HostAndPath)
		assert.Equal(t, "myimage:5.4.3", targetImages[2].NameAndTag)
	})

	t.Run("InvalidImageReference", func(t *testing.T) {
		t.Parallel()
		// given
		targetImage := &imagerewrite.TargetImage{}
		// when
		err := targetImage.From("invalid-image-reference")
		// then
		require.Error(t, err)
		require.ErrorIs(t, err, imagerewrite.ErrMissingSlashInImageReference)
		assert.Empty(t, targetImage.HostAndPath)
		assert.Empty(t, targetImage.NameAndTag)
	})
}
