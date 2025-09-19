package imagerewrite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/imagerewrite"
)

func TestPodContainerImageRewriter(t *testing.T) {
	t.Parallel()

	t.Run("RewriteSingleContainerImagesWithoutEnvs", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsImageReferences(reorder(13, []string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
		}))
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerImgRewriter := imagerewrite.PodContainerImageRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentNoEnvsContainer)
		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later
		containerResource := getFirstContainer(t, deploymentResource)
		require.NoError(t, err, "Failed to get first container from deployment resource")

		// when
		err = containerImgRewriter.Rewrite(targetImages, containerResource)
		require.NoError(t, err, "Failed to rewrite container images")
		err = setFirstContainer(t, deploymentResource, containerResource)
		require.NoError(t, err, "Failed to set first container in deployment resource")

		// then
		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the image in the container",
			).shouldChangeFrom("image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("image: private-registry.com/prod/template-operator:1.0.3"),
		)
	})

	t.Run("RewriteSingleContainerWithEnvs", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsImageReferences(reorder(17, []string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		}))
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerImgRewriter := imagerewrite.PodContainerImageRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainerWithEnvs)
		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later
		containerResource := getFirstContainer(t, deploymentResource)
		require.NoError(t, err, "Failed to get first container from deployment resource")

		// when
		err = containerImgRewriter.Rewrite(targetImages, containerResource)
		require.NoError(t, err, "Failed to rewrite container images")
		err = setFirstContainer(t, deploymentResource, containerResource)
		require.NoError(t, err, "Failed to set first container in deployment resource")

		// then
		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the image in the container",
			).shouldChangeFrom("image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("image: private-registry.com/prod/template-operator:1.0.3"),
		)
	})

	t.Run("RewriteSingleContainerImagesWithNoMatchingImages", func(t *testing.T) {
		t.Parallel()
		// given
		targetImagesThatDoNotMatch, err := imagerewrite.AsImageReferences(reorder(37, []string{
			"private-registry.com/prod/template-operator:1.0.4",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:3.2.1",
		}))
		require.NoError(t, err, "Failed to create target images from provided image references")
		containerImgRewriter := imagerewrite.PodContainerImageRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainerWithEnvs)
		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later
		containerResource := getFirstContainer(t, deploymentResource)
		require.NoError(t, err, "Failed to get first container from deployment resource")

		// when
		err = containerImgRewriter.Rewrite(targetImagesThatDoNotMatch, containerResource)
		require.NoError(t, err, "Failed to rewrite container images")
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

	t.Run("RewriteSingleContainerWithoutImageAttribute", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, _ := imagerewrite.AsImageReferences(reorder(37, []string{
			"private-registry.com/prod/template-operator:1.0.4",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:3.2.1",
		}))

		containerImgRewriter := imagerewrite.PodContainerImageRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainerNoImage)
		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later
		containerResource := getFirstContainer(t, deploymentResource)
		require.NoError(t, err, "Failed to get first container from deployment resource")

		// when
		err = containerImgRewriter.Rewrite(targetImages, containerResource)
		require.NoError(t, err, "Failed to rewrite container images")
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
