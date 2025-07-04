package imagerewrite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/declarative/v2/imagerewrite"
)

func TestResourceRewriter(t *testing.T) {
	t.Parallel()

	resourceRewriter := (&imagerewrite.ResourceRewriter{}).WithRewriters(
		&imagerewrite.PodContainerImageRewriter{},
		&imagerewrite.PodContainerEnvsRewriter{},
	)

	t.Run("SingleContainerRewriteAll", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainer)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)
		diffPos := diffLines(expectedLines, actualLines)

		assert.Len(t, diffPos, 3, "Expected 3 lines to be changed in the rewritten YAML")
		assert.Contains(t, expectedLines[diffPos[0]-1], "value: localhost:5000/foo-image:1.2.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[0]-1], "value: private-registry.com/prod/foo-image:1.2.3", "Actual rewritten image")

		assert.Contains(t, expectedLines[diffPos[1]-1], "value: example.com/myrepo/bar-image:4.5.6", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[1]-1], "value: private-registry.com/prod/bar-image:4.5.6", "Actual rewritten image")

		assert.Contains(t, expectedLines[diffPos[2]-1], "image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[2]-1], "image: private-registry.com/prod/template-operator:1.0.3", "Actual rewritten image")
	})

	t.Run("SingleContainerWithoutEnvsRewriteAll", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentNoEnvContainer)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)
		diffPos := diffLines(expectedLines, actualLines)

		assert.Len(t, diffPos, 1, "Expected 3 lines to be changed in the rewritten YAML")
		assert.Contains(t, expectedLines[diffPos[0]-1], "image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[0]-1], "image: private-registry.com/prod/template-operator:1.0.3", "Actual rewritten image")
	})

	t.Run("SingleContainerRewriteSome", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainer)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)
		diffPos := diffLines(expectedLines, actualLines)

		assert.Len(t, diffPos, 2, "Expected 2 lines to be changed in the rewritten YAML")

		assert.Contains(t, expectedLines[diffPos[0]-1], "value: example.com/myrepo/bar-image:4.5.6", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[0]-1], "value: private-registry.com/prod/bar-image:4.5.6", "Actual rewritten image")

		assert.Contains(t, expectedLines[diffPos[1]-1], "image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[1]-1], "image: private-registry.com/prod/template-operator:1.0.3", "Actual rewritten image")
	})

	t.Run("DoubleContainerRewriteSome", func(t *testing.T) {
		t.Parallel()
		// given
		localizedImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/bar-image:4.5.6",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		deploymentResource, err := parseToUnstructured(testDeploymentDoubleContainer)

		require.NoError(t, err, "Failed to parse test deployment to unstructured")
		unmodifiedYAML := mustYAML(deploymentResource) // Store the original YAML for comparison later

		// when
		err = resourceRewriter.ReplaceImages(deploymentResource, localizedImages)
		// then
		require.NoError(t, err, "Failed to rewrite container images")

		rewrittenYAML := mustYAML(deploymentResource)
		expectedLines := asLines(unmodifiedYAML)
		actualLines := asLines(rewrittenYAML)
		diffPos := diffLines(expectedLines, actualLines)

		assert.Len(t, diffPos, 4, "Expected 2 lines to be changed in the rewritten YAML")

		// Env in the first container
		assert.Contains(t, expectedLines[diffPos[0]-1], "value: example.com/myrepo/bar-image:4.5.6", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[0]-1], "value: private-registry.com/prod/bar-image:4.5.6", "Actual rewritten image")

		// Image in the first container
		assert.Contains(t, expectedLines[diffPos[1]-1], "image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[1]-1], "image: private-registry.com/prod/template-operator:1.0.3", "Actual rewritten image")

		// Env in the second container
		assert.Contains(t, expectedLines[diffPos[2]-1], "value: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[2]-1], "value: private-registry.com/prod/template-operator:1.0.3", "Actual rewritten image")

		// Image in the second container
		assert.Contains(t, expectedLines[diffPos[3]-1], "image: example.com/myrepo/bar-image:4.5.6", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[3]-1], "image: private-registry.com/prod/bar-image:4.5.6", "Actual rewritten image")
	})
}

const testDeploymentNoEnvContainer = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: template-operator-controller-manager
  namespace: template-operator-system
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: template-operator.kyma-project.io
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/default-container: manager
      labels:
        app.kubernetes.io/component: template-operator.kyma-project.io
    spec:
      containers:
      - args:
        - --leader-elect
        imagePullPolicy: Always
        image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3
        command:
        - /manager
`

const testDeploymentSingleContainer = testDeploymentNoEnvContainer + `
        env:
        - name: FOO
          value: localhost:5000/foo-image:1.2.3
        - name: BAR
          value: example.com/myrepo/bar-image:4.5.6
`

// testDeploymentDoubleContainer is a deployment with two containers, one of which is the same as in testDeploymentSingleContainer.
const testDeploymentDoubleContainer = testDeploymentSingleContainer + `
      - args:
        - --leader-elect
        env:
        - name: QUX
          value: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3
        - name: BAZ
          value: example.com/myrepo/baz-image:4.5.6
        imagePullPolicy: Always
        image: example.com/myrepo/bar-image:4.5.6
        command:
        - /manager
`
