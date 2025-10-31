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
			"myimage:1.2.3",
			"myimage:1.2.3@sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			"myimage@sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			"example.com/myapp/myimage:1.2.3",
			"example.com/myapp/myimage:1.2.3@sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			"example.com/myapp/myimage@sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			"example.com:5000/myapp/myimage:3.2.1",
			"example.com:5000/myapp/myimage:3.2.1" +
				"@sha256:9a1de2363c531f585a01e185095498d700fcd10fc8801577e5e4e262832dd3cd",
			"example.com:5000/myapp/myimage@sha256:9a1de2363c531f585a01e185095498d700fcd10fc8801577e5e4e262832dd3cd",
			"localhost:5111/myimage:5.4.3",
			"localhost:5111/myimage:5.4.3@sha256:f9f4a45fe9091a8e55b55b80241c522b45a66501703728d386dc4171f70af803",
			"localhost:5111/myimage@sha256:f9f4a45fe9091a8e55b55b80241c522b45a66501703728d386dc4171f70af803",
			"example.pkg.io/example-project/prod/external/gcr.io/kaniko-project/executor:v1.24.0",
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
				name:                "Image without host and no digest",
				expectedHostAndPath: "docker.io/library",
				expectedNameAndTag:  "myimage:1.2.3",
				expectedDigest:      "",
			},
			{
				name:                "Image without host with digest",
				expectedHostAndPath: "docker.io/library",
				expectedNameAndTag:  "myimage:1.2.3",
				expectedDigest:      "sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			},
			{
				name:                "Image without host and tag and with digest",
				expectedHostAndPath: "docker.io/library",
				expectedNameAndTag:  "myimage",
				expectedDigest:      "sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
			},
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
				name:                "Image without port and tag and with digest",
				expectedHostAndPath: "example.com/myapp",
				expectedNameAndTag:  "myimage",
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
				expectedDigest:      "sha256:9a1de2363c531f585a01e185095498d700fcd10fc8801577e5e4e262832dd3cd",
			},
			{
				name:                "Image with port and without tag and digest",
				expectedHostAndPath: "example.com:5000/myapp",
				expectedNameAndTag:  "myimage",
				expectedDigest:      "sha256:9a1de2363c531f585a01e185095498d700fcd10fc8801577e5e4e262832dd3cd",
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
			{
				name:                "Image with localhost and digest without tag",
				expectedHostAndPath: "localhost:5111",
				expectedNameAndTag:  "myimage",
				expectedDigest:      "sha256:f9f4a45fe9091a8e55b55b80241c522b45a66501703728d386dc4171f70af803",
			},
			{
				name:                "Image with paths are allowed and dots in path",
				expectedHostAndPath: "example.pkg.io/example-project/prod/external/gcr.io/kaniko-project",
				expectedNameAndTag:  "executor:v1.24.0",
				expectedDigest:      "",
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
				name:        "Uppercase letter in name",
				strValue:    "example.com/myapp/MyImage:1.2.3",
				expectedErr: imagerewrite.ErrInvalidImageReference,
			},
			{
				name:        "Invalid tag start character",
				strValue:    "myimage:-invalid-tag",
				expectedErr: imagerewrite.ErrInvalidImageReference,
			},
			{
				name:        "Invalid digest format, digest is too short",
				strValue:    "myimage@sha256:12345",
				expectedErr: imagerewrite.ErrInvalidImageReference,
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
				name: "Image with digest",
				strValue: "example.com/myapp/myimage:1.2.3" +
					"@sha256:837eb50a66bc0915d1986d376920c400d5db18075204339c0b047f5ba2091aa5",
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
