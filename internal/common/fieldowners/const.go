package fieldowners

import "sigs.k8s.io/controller-runtime/pkg/client"

const (
	LifecycleManager = client.FieldOwner("operator.kyma-project.io/lifecycle-manager")
)
