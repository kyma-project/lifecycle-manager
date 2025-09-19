package imagerewrite_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/internal/imagerewrite"
)

// testDeploymentNoEnvsContainer is a deployment with a single container that has no environment variables set.
const testDeploymentNoEnvsContainer = `
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
        thisShouldntBeChanged: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3
    spec:
      containers:
      - name: manager
        args:
        - --leader-elect
        imagePullPolicy: Always
        image: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3
        command:
        - /manager
`

// testDeploymentSingleContainerWithEnvs is a deployment with a single container that has two environment variables set.
const testDeploymentSingleContainerWithEnvs = testDeploymentNoEnvsContainer + `
        env:
        - name: FOO
          value: localhost:5000/foo-image:1.2.3
        - name: BAR
          value: example.com/myrepo/bar-image:4.5.6
        - name: TEST_NAMESPACE_A
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
`

// testDeploymentTwoContainers is a deployment with two containers, both having environment variables set.
const testDeploymentTwoContainers = testDeploymentSingleContainerWithEnvs + `
      - name: second-container
        args:
        - --leader-elect
        env:
        - name: QUX
          value: europe-docker.pkg.dev/second-container/env/qux-image:1.2.3
        - name: BAZ
          value: example.com/second-container/baz-image:4.5.6
        - name: TEST_NAMESPACE_B
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        image: example.com/second-container/operator-image:7.8.9
        command:
        - /manager
        imagePullPolicy: Always
`

// testDeploymentWithInitContainer is a deployment with a single container and an init container,
// both having environment variables set.
const testDeploymentWithInitContainer = testDeploymentSingleContainerWithEnvs + `
      initContainers:
      - name: init-container
        image: example.com/myrepo/init-image:1.0.0
        command:
        - /init-script.sh
        args:
        - --arg1=value1
        env:
        - name: QUX
          value: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3
        - name: BAZ
          value: example.com/myrepo/baz-image:4.5.6
        - name: TEST_NAMESPACE_C
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        imagePullPolicy: Always
`

// testDeploymentTwoContainersWithInitContainer is a deployment with two containers and an init container,
// all having environment variables set.
const testDeploymentTwoContainersWithInitContainer = testDeploymentTwoContainers + `
      initContainers:
      - name: init-container
        image: example.com/somerepo/other-init-image:1.1.1
        command:
        - /init-script.sh
        args:
        - --arg1=value1
        env:
        - name: TEST_NAMESPACE_D
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: DB_CONTAINER
          value: example.com/sql/db-image:0.3.1
        - name: MQ_CONTAINER
          value: example.com/message/queue-impl:1.3.0
        imagePullPolicy: Always
`

// testDeploymentSingleContainerNoImage is a deployment with a single container that has no "image" attribute.
const testDeploymentSingleContainerNoImage = `
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
        thisShouldntBeChanged: europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3
    spec:
      containers:
      - name: manager
        args:
        - --leader-elect
        imagePullPolicy: Always
        command:
        - /manager
`

const testCronJob = `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: hello
spec:
  schedule: "* * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox:1.28
            imagePullPolicy: IfNotPresent
            command:
            - /bin/sh
            - -c
            - date; echo Hello from the Kubernetes cluster
          restartPolicy: OnFailure
`

// changesComparator is used to make comparison between original and rewritten object in a YAML format easier.
// It compares YAMLs line-by-line, assuming that the overall structure is identical.
type changesComparator struct {
	t              *testing.T
	originalLines  []string
	rewrittenLines []string
	// verifiers     []*verifer
	differences []int
}

func newChangesComparator(t *testing.T, originalLines, rewrittenLines []string) *changesComparator {
	t.Helper()
	require.Len(
		t,
		originalLines,
		len(rewrittenLines),
		"Original lines count (%d) should equal the rewritten lines count (%d)",
		len(originalLines),
		len(rewrittenLines),
	)

	return &changesComparator{
		originalLines:  originalLines,
		rewrittenLines: rewrittenLines,
		differences:    diffLines(originalLines, rewrittenLines),
		t:              t,
	}
}

// verify checks that the expected changes exist in the differences between expected and actual lines.
func (c *changesComparator) verify(expectedChanges ...(func() *changeDefiner)) {
	definersCount := len(expectedChanges)
	changesCount := len(c.differences)
	require.Len(
		c.t,
		expectedChanges,
		changesCount,
		"The number of registered changes (%d) must be equal to the number of differences between lines (%d)",
		definersCount,
		changesCount,
	)

	for _, lineNumber := range c.differences {
		require.GreaterOrEqual(
			c.t,
			lineNumber,
			0,
			"The difference position %d should be greater than or equal to 0",
			lineNumber,
		)
		require.Less(
			c.t,
			lineNumber,
			len(c.originalLines),
			"The difference position %d should be less than the number of lines in comparison (%d)",
			lineNumber,
			len(c.originalLines),
		)
	}

	for i, cd := range expectedChanges {
		differenceIndex := c.differences[i]
		originalLine := c.originalLines[differenceIndex]
		rewrittenLine := c.rewrittenLines[differenceIndex]
		changeDefiner := cd()
		originalValueShouldContain := changeDefiner.shouldChangeFromValue
		rewrittenValueShouldContain := changeDefiner.toValue

		assert.Contains(
			c.t,
			originalLine,
			originalValueShouldContain,
			"Problem with %s\n: => Original line %q does not contain the configured value: %q",
			changeDefiner.description,
			originalLine,
			originalValueShouldContain,
		)
		assert.Contains(
			c.t,
			rewrittenLine,
			rewrittenValueShouldContain,
			"Problem with %s\n: => Rewriten line %q does not contain the configured value: %q",
			changeDefiner.description,
			rewrittenLine,
			rewrittenValueShouldContain,
		)
	}
}

// changeDefiner is used to "register" a change to be verified later on.
// The API allows to define a change in a readable, DSL way.
type changeDefiner struct {
	description           string
	shouldChangeFromValue string
	toValue               string
}

func valueOf(desc string) *changeDefiner {
	return &changeDefiner{
		description: desc,
	}
}

func (cd *changeDefiner) shouldChangeFrom(value string) *changeDefiner {
	cd.shouldChangeFromValue = value
	return cd
}

func (cd *changeDefiner) to(value string) func() *changeDefiner {
	cd.toValue = value
	return func() *changeDefiner {
		return cd
	}
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

// diffLines compares two slices of strings line by line and returns the indices of lines that differ.
// Both slices must have the same length.
func diffLines(lines1, lines2 []string) []int {
	if len(lines1) != len(lines2) {
		panic(fmt.Sprintf("line counts do not match: %d vs %d", len(lines1), len(lines2)))
	}

	diff := make([]int, 0)

	for i := range lines1 {
		if lines1[i] != lines2[i] {
			diff = append(diff, i)
		}
	}
	return diff
}

// getFirstContainer retrieves the first container from a deployment-like resource.
func getFirstContainer(t *testing.T, deployment *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	res, err := imagerewrite.GetPodContainers(deployment)
	require.NoError(t, err, "Failed to get containers from deployment resource")
	return res[0]
}

// setFirstContainer replaces the first container in a deployment-like resource with the provided container.
func setFirstContainer(
	t *testing.T,
	deployment *unstructured.Unstructured,
	container *unstructured.Unstructured,
) error {
	t.Helper()
	containers, err := imagerewrite.GetPodContainers(deployment)
	require.NoError(t, err, "Failed to get containers from deployment resource")
	containers[0] = container
	return imagerewrite.SetPodContainers(deployment, containers)
}

// reorder is a helper function to reorder the input slice to
// reduce the likelihood that the order of elements matters in the test.
func reorder(seed int, input []string) []string {
	for i := range input {
		firstIndex := i
		secondIndex := (i*3 + seed) % len(input)
		input[firstIndex], input[secondIndex] = input[secondIndex], input[firstIndex]
	}
	return input
}
