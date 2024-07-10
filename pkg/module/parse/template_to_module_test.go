package parse_test

import (
	"context"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/img"
	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func Test_ReadAssociatedResourcesField(t *testing.T) {
	layerRepresentation := img.OCI{
		Type: string(v1beta2.OciRefType),
		Ref:  "testdata/sha256:89256dd62ebab4f7f02482084b91301087a084426752b00754e14afd3575e019",
		Name: "test-name",
		Repo: "test-repo",
	}
	layer := img.Layer{
		LayerName:           img.AssociatedResourcesLayer,
		LayerRepresentation: &layerRepresentation,
	}
	got, err := parse.ReadAssociatedResourcesField(context.TODO(), layer)
	assert.NoError(t, err)
	expectedResources := []string{
		"serverless.kyma-project.io/v1alpha2/functions",
		"operator.kyma-project.io/v1alpha1/serverlesses",
	}
	if !reflect.DeepEqual(got, expectedResources) {
		t.Errorf("readAssociatedResourcesField() got = %v, want %v", got, expectedResources)
	}
}
