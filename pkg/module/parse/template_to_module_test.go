package parse_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
)

func TestFetchAssociatedResources(t *testing.T) {
	associatedResourcesFileName := "testdata/associated-resources"

	associatedResourcesFile, err := os.Open(associatedResourcesFileName)
	require.NoError(t, err)

	got, err := parse.ReadAssociatedResources(associatedResourcesFile)
	require.NoError(t, err)

	expectedResources := []string{
		"serverless.kyma-project.io/v1alpha2/functions",
		"operator.kyma-project.io/v1alpha1/serverlesses",
	}
	if !reflect.DeepEqual(got, expectedResources) {
		t.Errorf("FetchAssociatedResources() got = %v, want %v", got, expectedResources)
	}
}
