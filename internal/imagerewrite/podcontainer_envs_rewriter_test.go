package imagerewrite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/imagerewrite"
)

func TestPodContainerEnvRewriter(t *testing.T) {
	t.Parallel()

	t.Run("RewriteSingleContainerEnvsHappyPath", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/stage/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerEnvsRewriter := imagerewrite.PodContainerEnvsRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainerWithEnvs)
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

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the first env var in the container",
			).shouldChangeFrom("localhost:5000/foo-image:1.2.3").
				to("private-registry.com/prod/foo-image:1.2.3"),
			valueOf(
				"the second env var in the container",
			).shouldChangeFrom("example.com/myrepo/bar-image:4.5.6").
				to("private-registry.com/stage/bar-image:4.5.6"),
		)
	})

	t.Run("RewriteSingleContainerEnvsWithNoMatchingImages", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/prod/foo-image:10.20.30",
			"private-registry.com/stage/bar-image:6.5.4",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerEnvsRewriter := imagerewrite.PodContainerEnvsRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainerWithEnvs)
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
		assert.YAMLEq(
			t,
			unmodifiedYAML,
			rewrittenYAML,
			"Expected no changes in the rewritten YAML",
		)
	})

	t.Run("RewriteSingleContainerWithoutEnvs", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/prod/foo-image:10.20.30",
			"private-registry.com/stage/bar-image:6.5.4",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerEnvsRewriter := imagerewrite.PodContainerEnvsRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentNoEnvsContainer)
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
		assert.YAMLEq(
			t,
			unmodifiedYAML,
			rewrittenYAML,
			"Expected no changes in the rewritten YAML",
		)
	})
}
