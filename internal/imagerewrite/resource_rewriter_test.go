package imagerewrite_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/imagerewrite"
)

func TestResourceRewriter(t *testing.T) {
	t.Parallel()

	resourceRewriter := (&imagerewrite.ResourceRewriter{}).WithRewriters(
		&imagerewrite.PodContainerImageRewriter{},
		&imagerewrite.PodContainerEnvsRewriter{},
	)

	t.Run("UnsupportedKind", func(t *testing.T) {
		t.Parallel()
		// given
		cronJobResource, err := parseToUnstructured(testCronJob)
		require.NoError(t, err, "Failed to parse test ConfigMap to unstructured")
		unmodifiedYAML := mustYAML(cronJobResource) // Store the original YAML for comparison later
		// when
		err = resourceRewriter.ReplaceImages(cronJobResource, nil)
		// then
		require.NoError(t, err, "Unexpected error when re-writing unsupported resource kind")
		rewrittenYAML := mustYAML(cronJobResource)

		require.YAMLEq(
			t,
			unmodifiedYAML,
			rewrittenYAML,
			"ConfigMap should not be modified",
		)
	})

	t.Run("SingleContainerRewriteAll", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainerWithEnvs)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the first env var in the first container",
			).shouldChangeFrom("value: localhost:5000/foo-image:1.2.3").
				to("value: private-registry.com/prod/foo-image:1.2.3"),
			valueOf(
				"the second env var in the first container",
			).shouldChangeFrom("value: example.com/myrepo/bar-image:4.5.6").
				to("value: private-registry.com/prod/bar-image:4.5.6"),
			valueOf(
				"the image in the first container",
			).shouldChangeFrom("image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("image: private-registry.com/prod/template-operator:1.0.3"),
		)
	})

	t.Run("SingleContainerWithoutEnvsRewriteAll", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentNoEnvsContainer)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

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

	t.Run("SingleContainerRewriteSome", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainerWithEnvs)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the second env var in the container",
			).shouldChangeFrom("value: example.com/myrepo/bar-image:4.5.6").
				to("value: private-registry.com/prod/bar-image:4.5.6"),
			valueOf(
				"the image in the container",
			).shouldChangeFrom("image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("image: private-registry.com/prod/template-operator:1.0.3"),
		)
	})

	t.Run("DoubleContainerRewriteSome", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/dev/bar-image:4.5.6",
			"private-registry.com/prod/qux-image:1.2.3",
			"private-registry.com/stage/operator-image:7.8.9",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentTwoContainers)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the second env var in the first container",
			).shouldChangeFrom("value: example.com/myrepo/bar-image:4.5.6").
				to("value: private-registry.com/dev/bar-image:4.5.6"),
			valueOf(
				"the image in the first container",
			).shouldChangeFrom("image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("image: private-registry.com/prod/template-operator:1.0.3"),
			valueOf(
				"the first env var in the second container",
			).shouldChangeFrom("value: europe-docker.pkg.dev/second-container/env/qux-image:1.2.3").
				to("value: private-registry.com/prod/qux-image:1.2.3"),
			valueOf(
				"the image in the second container",
			).shouldChangeFrom("image: example.com/second-container/operator-image:7.8.9").
				to("image: private-registry.com/stage/operator-image:7.8.9"),
		)
	})

	t.Run("InitContainerRewriteAll", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsImageReferences([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
			"very-private-registry.com/base/init-image:1.0.0",
			"very-private-registry.com/baz/baz-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentWithInitContainer)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the first env var in the first container",
			).shouldChangeFrom("value: localhost:5000/foo-image:1.2.3").
				to("value: private-registry.com/prod/foo-image:1.2.3"),
			valueOf(
				"the second env var in the first container",
			).shouldChangeFrom("value: example.com/myrepo/bar-image:4.5.6").
				to("value: private-registry.com/prod/bar-image:4.5.6"),
			valueOf(
				"the image in the first container",
			).shouldChangeFrom("image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("image: private-registry.com/prod/template-operator:1.0.3"),
			valueOf(
				"the first env var in the init container",
			).shouldChangeFrom("value: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("value: private-registry.com/prod/template-operator:1.0.3"),
			valueOf(
				"the second env var in the init container",
			).shouldChangeFrom("value: example.com/myrepo/baz-image:4.5.6").
				to("value: very-private-registry.com/baz/baz-image:4.5.6"),
			valueOf(
				"the image in the init container",
			).shouldChangeFrom("image: example.com/myrepo/init-image:1.0.0").
				to("image: very-private-registry.com/base/init-image:1.0.0"),
		)
	})

	t.Run("DeploymentTwoContainersWithInitContainerRewriteAll", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsImageReferences(reorder(59, []string{
			"localhost:5123/foo-image:1.2.3",
			"private-registry.com/first/container/bar-image:4.5.6",
			"private-registry.com:123/first/container/template-operator:1.0.3",
			"very-private-registry.com:123/second/container/qux-image:1.2.3",
			"very-private-registry.com/second/container/baz-image:4.5.6",
			"very-private-registry.com/second/container/operator-image:7.8.9",
			"really-private-registry.com/first/init-container/db-image:0.3.1",
			"really-private-registry.com/first/init-container/queue-impl:1.3.0",
			"really-private-registry.com/first/init-container/other-init-image:1.1.1",
			"private-registry.com/stage/other-operator:1.2.3",
		}))

		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentTwoContainersWithInitContainer)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)

		cp := newChangesComparator(t, expectedLines, actualLines)
		cp.verify(
			valueOf(
				"the first env var in the first container",
			).shouldChangeFrom("value: localhost:5000/foo-image:1.2.3").
				to("value: localhost:5123/foo-image:1.2.3"),
			valueOf(
				"the second env var in the first container",
			).shouldChangeFrom("value: example.com/myrepo/bar-image:4.5.6").
				to("value: private-registry.com/first/container/bar-image:4.5.6"),
			valueOf(
				"the image in the first container",
			).shouldChangeFrom("image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3").
				to("image: private-registry.com:123/first/container/template-operator:1.0.3"),
			valueOf(
				"the first env var in the second container",
			).shouldChangeFrom("value: europe-docker.pkg.dev/second-container/env/qux-image:1.2.3").
				to("value: very-private-registry.com:123/second/container/qux-image:1.2.3"),
			valueOf(
				"the second env var in the second container",
			).shouldChangeFrom("value: example.com/second-container/baz-image:4.5.6").
				to("value: very-private-registry.com/second/container/baz-image:4.5.6"),
			valueOf(
				"the image in the second container",
			).shouldChangeFrom("image: example.com/second-container/operator-image:7.8.9").
				to("image: very-private-registry.com/second/container/operator-image:7.8.9"),
			valueOf(
				"the first env var in the init container",
			).shouldChangeFrom("value: example.com/sql/db-image:0.3.1").
				to("value: really-private-registry.com/first/init-container/db-image:0.3.1"),
			valueOf(
				"the second env var in the init container",
			).shouldChangeFrom("value: example.com/message/queue-impl:1.3.0").
				to("value: really-private-registry.com/first/init-container/queue-impl:1.3.0"),
			valueOf(
				"the image in the init container",
			).shouldChangeFrom("image: example.com/somerepo/other-init-image:1.1.1").
				to("image: really-private-registry.com/first/init-container/other-init-image:1.1.1"),
		)
	})
}
