package parse_test

import (
	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
	"github.com/stretchr/testify/require"
	"os"
	"reflect"
	"testing"
)

func TestFetchAssociatedResources(t *testing.T) {
	associatedResourcesFileName := "testdata/associated-resources"

	associatedResourcesFile, err := os.Open(associatedResourcesFileName)
	require.NoError(t, err)

	got, err := parse.FetchAssociatedResources(associatedResourcesFile)
	expectedResources := []string{
		"serverless.kyma-project.io/v1alpha2/functions",
		"operator.kyma-project.io/v1alpha1/serverlesses",
	}
	if !reflect.DeepEqual(got, expectedResources) {
		t.Errorf("FetchAssociatedResources() got = %v, want %v", got, expectedResources)
	}

}
