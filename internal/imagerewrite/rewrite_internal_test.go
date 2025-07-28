package imagerewrite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	GetPodContainers = getPodContainers // exported for testing only
	SetPodContainers = setPodContainers // exported for testing only
)

func TestIsLikelyImageReference(t *testing.T) {
	t.Run("StandardImageReference", func(t *testing.T) {
		// given
		imageRef := "example.com/myapp/myimage:1.2.3"
		// when
		isImage := isImageRefForReplacement(imageRef, NameAndTag("myimage:1.2.3"))
		// then
		assert.True(t, isImage, "Expected %s to be a valid image reference", imageRef)
	})

	t.Run("LocalhostImageReference", func(t *testing.T) {
		// given
		imageRef := "localhost:5000/myimage:1.2.3"
		// when
		isImage := isImageRefForReplacement(imageRef, "myimage:1.2.3")
		// then
		assert.True(t, isImage, "Expected %s to be a valid image reference", imageRef)
	})

	t.Run("NoPathImageReference", func(t *testing.T) {
		// given
		imageRef := "example.com:6789/myimage:1.2.3"
		// when
		isImage := isImageRefForReplacement(imageRef, "myimage:1.2.3")
		// then
		assert.True(t, isImage, "Expected %s to be a valid image reference", imageRef)
	})

	t.Run("ImageReferenceThatShouldBeIgnored", func(t *testing.T) {
		// given
		imageRef := "example.com/myapp/myimage:1.2.3"
		// when
		isImage := isImageRefForReplacement(imageRef, "myimage:4.3.2")
		// then
		assert.False(t, isImage, "Expected %s to be a valid image reference", imageRef)
	})
}
