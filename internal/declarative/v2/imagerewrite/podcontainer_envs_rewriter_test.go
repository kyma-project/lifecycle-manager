package imagerewrite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/declarative/v2/imagerewrite"
)

func TestPodContainerEnvRewriter(t *testing.T) {
	t.Parallel()

	t.Run("RewriteSingleContainerEnvsHappyPath", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/stage/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerEnvsRewriter := imagerewrite.PodContainerEnvsRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainer)
		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later
		containerResource := getFirstContainer(t, deploymentResource)
		require.NoError(t, err, "Failed to get first container from deployment resource")

		// when
		err = containerEnvsRewriter.Rewrite(targetImages, containerResource)
		require.NoError(t, err, "Failed to rewrite container envs")
		err = setFirstContainer(t, deploymentResource, containerResource)
		require.NoError(t, err, "Failed to set first container in deployment resource")

		// then
		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)
		diffPos := diffLines(expectedLines, actualLines)

		assert.Len(t, diffPos, 2, "Expected 2 lines to be changed in the rewritten YAML")
		assert.Contains(t, expectedLines[diffPos[0]-1], "value: localhost:5000/foo-image:1.2.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[0]-1], "value: private-registry.com/prod/foo-image:1.2.3", "Actual rewritten image")

		assert.Contains(t, expectedLines[diffPos[1]-1], "value: example.com/myrepo/bar-image:4.5.6", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[1]-1], "value: private-registry.com/stage/bar-image:4.5.6", "Actual rewritten image")
	})

	t.Run("RewriteSingleContainerEnvsWithNoMatchingImages", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/prod/foo-image:10.20.30",
			"private-registry.com/stage/bar-image:6.5.4",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerEnvsRewriter := imagerewrite.PodContainerEnvsRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainer)
		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later
		containerResource := getFirstContainer(t, deploymentResource)
		require.NoError(t, err, "Failed to get first container from deployment resource")

		// when
		err = containerEnvsRewriter.Rewrite(targetImages, containerResource)
		require.NoError(t, err, "Failed to rewrite container envs")
		err = setFirstContainer(t, deploymentResource, containerResource)
		require.NoError(t, err, "Failed to set first container in deployment resource")

		// then
		rewrittenYAML := mustYAML(deploymentResource)
		assert.Equal(t, unmodifiedYAML, rewrittenYAML, "Expected no changes in the rewritten YAML") //nolint: testifylint // I want to test for equality, not for equivalence
	})

	t.Run("RewriteSingleContainerWithoutEnvs", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/prod/foo-image:10.20.30",
			"private-registry.com/stage/bar-image:6.5.4",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerEnvsRewriter := imagerewrite.PodContainerEnvsRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentNoEnvContainer)
		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later
		containerResource := getFirstContainer(t, deploymentResource)
		require.NoError(t, err, "Failed to get first container from deployment resource")

		// when
		err = containerEnvsRewriter.Rewrite(targetImages, containerResource)
		require.NoError(t, err, "Failed to rewrite container envs")
		err = setFirstContainer(t, deploymentResource, containerResource)
		require.NoError(t, err, "Failed to set first container in deployment resource")

		// then
		rewrittenYAML := mustYAML(deploymentResource)
		assert.Equal(t, unmodifiedYAML, rewrittenYAML, "Expected no changes in the rewritten YAML") //nolint: testifylint // I want to test for equality, not for equivalence
	})
}
