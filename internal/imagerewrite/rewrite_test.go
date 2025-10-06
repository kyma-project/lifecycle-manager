package imagerewrite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/imagerewrite"
)

func TestDockerImageReference(t *testing.T) {
	t.Parallel()

	t.Run("ValidImageReferences", func(t *testing.T) {
		t.Parallel()
		// given
		imageRefs, err := imagerewrite.AsImageReferences([]string{
			"example.com/myapp/myimage:1.2.3",
			"example.com/myapp/myimage:1.2.3@sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			"example.com:5000/myapp/myimage:3.2.1",
			"example.com:5000/myapp/myimage:3.2.1@sha256:c140c4dcdfe38aa7b462d9173ff4bad8fbfbb4819c5d9398c53c50abec7",
			"localhost:5111/myimage:5.4.3",
			"localhost:5111/myimage:5.4.3@sha256:f9f4a45fe9091a8e55b55b80241c522b45a66501703728d386dc4171f70af803",
		})

		// then
		require.NoError(t, err)

		testCases := []struct {
			name                string
			expectedHostAndPath string
			expectedNameAndTag  string
			expectedDigest      string
		}{
			{
				name:                "Image without port and no digest",
				expectedHostAndPath: "example.com/myapp",
				expectedNameAndTag:  "myimage:1.2.3",
				expectedDigest:      "",
			},
			{
				name:                "Image without port with digest",
				expectedHostAndPath: "example.com/myapp",
				expectedNameAndTag:  "myimage:1.2.3",
				expectedDigest:      "sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			},
			{
				name:                "Image with port and no digest",
				expectedHostAndPath: "example.com:5000/myapp",
				expectedNameAndTag:  "myimage:3.2.1",
				expectedDigest:      "",
			},
			{
				name:                "Image with port and digest",
				expectedHostAndPath: "example.com:5000/myapp",
				expectedNameAndTag:  "myimage:3.2.1",
				expectedDigest:      "sha256:c140c4dcdfe38aa7b462d9173ff4bad8fbfbb4819c5d9398c53c50abec7",
			},
			{
				name:                "Image with localhost and no digest",
				expectedHostAndPath: "localhost:5111",
				expectedNameAndTag:  "myimage:5.4.3",
				expectedDigest:      "",
			},
			{
				name:                "Image with localhost and digest",
				expectedHostAndPath: "localhost:5111",
				expectedNameAndTag:  "myimage:5.4.3",
				expectedDigest:      "sha256:f9f4a45fe9091a8e55b55b80241c522b45a66501703728d386dc4171f70af803",
			},
		}

		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tc.expectedHostAndPath, imageRefs[i].HostAndPath)
				assert.Equal(t, tc.expectedNameAndTag, string(imageRefs[i].NameAndTag))
				assert.Equal(t, tc.expectedDigest, imageRefs[i].Digest)
			})
		}
	})

	t.Run("InvalidImageReferences", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name        string
			strValue    string
			expectedErr error
		}{
			{
				name:        "Missing colon in image reference",
				strValue:    "example.com/myapp/myimage",
				expectedErr: imagerewrite.ErrMissingColonInImageReference,
			},
			{
				name:        "Missing slash in image reference",
				strValue:    "example.commyappmyimage:1.2.3",
				expectedErr: imagerewrite.ErrMissingSlashInImageReference,
			},
		}
		for _, tcase := range testCases {
			t.Run(tcase.name, func(t *testing.T) {
				t.Parallel()

				// given, when
				targetImage, err := imagerewrite.NewDockerImageReference(tcase.strValue)

				// then
				require.Error(t, err)
				require.ErrorIs(t, err, tcase.expectedErr)
				require.Nil(t, targetImage)
			})
		}
	})

	t.Run("StringRepresentation", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name     string
			strValue string
		}{
			{
				name:     "Image without digest",
				strValue: "example.com/myapp/myimage:1.2.3",
			},
			{
				name:     "Image with digest",
				strValue: "example.com/myapp/myimage:1.2.3@sha256:837eb50a66bc0915d1986d376920c40b047f5ba2091aa5",
			},
		}

		for _, tcase := range testCases {
			t.Run(tcase.name, func(t *testing.T) {
				t.Parallel()

				imageRef, err := imagerewrite.NewDockerImageReference(tcase.strValue)
				require.NoError(t, err)
				strRepresentation := imageRef.String()
				assert.Equal(t, tcase.strValue, strRepresentation)
			})
		}
	})
}
