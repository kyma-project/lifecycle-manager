package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func Test_AnnotationHasExternalDependencies(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "operator.kyma-project.io/module-version", v1beta2.ModuleVersionAnnotation)
	assert.Equal(t, "operator.kyma-project.io/is-cluster-scoped", v1beta2.IsClusterScopedAnnotation)
	assert.Equal(t, "operator.kyma-project.io/custom-state-check", v1beta2.CustomStateCheckAnnotation)
	assert.Equal(t, "skr-domain", v1beta2.SKRDomainAnnotation)
}

func Test_LabelHasExternalDependencies(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "kyma-project.io/instance-id", v1beta2.InstanceIDLabel)
	assert.Equal(t, "operator.kyma-project.io/skip-reconciliation", v1beta2.SkipReconcileLabel)
	assert.Equal(t, "operator.kyma-project.io/internal", v1beta2.InternalLabel)
	assert.Equal(t, "operator.kyma-project.io/beta", v1beta2.BetaLabel)
	assert.Equal(t, "operator.kyma-project.io/sync", v1beta2.SyncLabel)
	assert.Equal(t, "operator.kyma-project.io/module-name", v1beta2.ModuleName)
	assert.Equal(t, "operator.kyma-project.io/kyma-name", v1beta2.KymaName)
	assert.Equal(t, "operator.kyma-project.io/channel", v1beta2.ChannelLabel)
	assert.Equal(t, "operator.kyma-project.io/managed-by", v1beta2.ManagedBy)
}
