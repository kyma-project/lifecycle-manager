package imagerewrite_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/internal/declarative/v2/imagerewrite"
)

func TestPodContainerImageRewriter(t *testing.T) {
	t.Parallel()

	t.Run("RewriteSingleContainerImageHappyPath", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerImgRewriter := imagerewrite.PodContainerImageRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainer)
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
		diffPos := diffLines(expectedLines, actualLines)

		assert.Len(t, diffPos, 1, "Expected 1 line to be changed in the rewritten YAML")
		assert.Contains(t, expectedLines[diffPos[0]-1], "image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[0]-1], "image: private-registry.com/prod/template-operator:1.0.3", "Actual rewritten image")
	})

	t.Run("RewriteSingleContainerImagesWithNoMatchingImages", func(t *testing.T) {
		t.Parallel()
		// given
		targetImagesThatDoNotMatch, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.4",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:10.20.30",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")
		containerImgRewriter := imagerewrite.PodContainerImageRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentSingleContainer)
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
		assert.Equal(t, unmodifiedYAML, rewrittenYAML, "Expected no changes in the rewritten YAML") //nolint: testifylint // I want to test for equality, not for equivalence
	})

	t.Run("RewriteSingleContainerImagesWithoutEnvs", func(t *testing.T) {
		t.Parallel()
		// given
		targetImages, err := imagerewrite.AsTargetImages([]string{
			"private-registry.com/prod/template-operator:1.0.3",
			"private-registry.com/stage/other-operator:1.2.3",
			"private-registry.com/prod/foo-image:1.2.3",
		})
		require.NoError(t, err, "Failed to create target images from provided image references")

		containerImgRewriter := imagerewrite.PodContainerImageRewriter{}

		deploymentResource, err := parseToUnstructured(testDeploymentNoEnvContainer)
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
		diffPos := diffLines(expectedLines, actualLines)

		assert.Len(t, diffPos, 1, "Expected 1 line to be changed in the rewritten YAML")
		assert.Contains(t, expectedLines[diffPos[0]-1], "image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3", "Expected image to be rewritten")
		assert.Contains(t, actualLines[diffPos[0]-1], "image: private-registry.com/prod/template-operator:1.0.3", "Actual rewritten image")
	})
}

func parseToUnstructured(resource string) (*unstructured.Unstructured, error) {
	asJSON, err := yaml.YAMLToJSON([]byte(nlnl(resource)))
	if err != nil {
		return nil, err
	}

	u := unstructured.Unstructured{}
	if err := u.UnmarshalJSON(asJSON); err != nil {
		return nil, err
	}
	return &u, nil
}

// nlnl == "no leading newlines".
func nlnl(input string) string {
	return strings.TrimLeft(input, "\n")
}

func asLines(input string) []string {
	return strings.Split(input, "\n")
}

// mustYAML converts an unstructured object to YAML format and panics if it fails.
func mustYAML(obj *unstructured.Unstructured) string {
	yamlData, err := yaml.Marshal(obj.Object)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal object to YAML: %v", err))
	}
	return nlnl(string(yamlData))
}

// diffLines compares two slices of strings line by line and returns the indices of lines that differ. Both slices must have the same length.
func diffLines(lines1, lines2 []string) []int {
	if len(lines1) != len(lines2) {
		panic(fmt.Sprintf("line counts do not match: %d vs %d", len(lines1), len(lines2)))
	}

	diff := make([]int, 0)

	for i := range lines1 {
		if lines1[i] != lines2[i] {
			diff = append(diff, i+1) // +1 to convert to 1-based index
		}
	}
	return diff
}

func getFirstContainer(t *testing.T, deployment *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	res, err := imagerewrite.GetPodContainers(deployment)
	require.NoError(t, err, "Failed to get containers from deployment resource")
	return res[0]
}

func setFirstContainer(t *testing.T, deployment *unstructured.Unstructured, container *unstructured.Unstructured) error {
	t.Helper()
	containers, err := imagerewrite.GetPodContainers(deployment)
	require.NoError(t, err, "Failed to get containers from deployment resource")
	containers[0] = container
	return imagerewrite.SetPodContainers(deployment, containers)
}
