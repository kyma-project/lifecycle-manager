package matcher

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	"github.com/stretchr/testify/require"
)

func TestCreateCRDMatcherFrom(t *testing.T) {
	names := "kymas.operator.kyma-project.io,manifest.operator.kyma-project.io"
	matcherFunc := CreateCRDMatcherFrom(names)

	crdBuilder := builder.NewCRDBuilder()
	kymaCrd := crdBuilder.WithName("kyma").Build()
	manifestCrd := crdBuilder.WithName("manifest").Build()
	watcherCrd := crdBuilder.WithName("watcher").Build()

	require.True(t, matcherFunc(kymaCrd))
	require.True(t, matcherFunc(manifestCrd))
	require.False(t, matcherFunc(watcherCrd))
}
