package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

func Test_AnnotationHasExternalDependencies(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "operator.kyma-project.io/is-cluster-scoped", shared.IsClusterScopedAnnotation)
	assert.Equal(t, "operator.kyma-project.io/custom-state-check", shared.CustomStateCheckAnnotation)
	assert.Equal(t, "skr-domain", shared.SkrDomainAnnotation)
}

func Test_LabelHasExternalDependencies(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "kyma-project.io/instance-id", shared.InstanceIDLabel)
	assert.Equal(t, "operator.kyma-project.io/skip-reconciliation", shared.SkipReconcileLabel)
	assert.Equal(t, "operator.kyma-project.io/internal", shared.InternalLabel)
	assert.Equal(t, "operator.kyma-project.io/beta", shared.BetaLabel)
	assert.Equal(t, "operator.kyma-project.io/module-name", shared.ModuleName)
	assert.Equal(t, "operator.kyma-project.io/kyma-name", shared.KymaName)
	assert.Equal(t, "operator.kyma-project.io/channel", shared.ChannelLabel)
	assert.Equal(t, "operator.kyma-project.io/managed-by", shared.ManagedBy)
	assert.Equal(t, "istio-injection", shared.IstioInjectionLabel)
	assert.Equal(t, "namespaces.warden.kyma-project.io/validate", shared.WardenLabel)
}
