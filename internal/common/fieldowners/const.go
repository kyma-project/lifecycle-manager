package fieldowners

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	LifecycleManager        = client.FieldOwner("operator.kyma-project.io/lifecycle-manager")
	LegacyLifecycleManager  = client.FieldOwner(shared.OperatorName)
	CustomResourceFinalizer = client.FieldOwner("resource.kyma-project.io/finalizer")
	DeclarativeApplier      = client.FieldOwner("declarative.kyma-project.io/applier")
	ModuleCatalogSync       = client.FieldOwner("catalog-sync")
	KymaSyncContextProvider = client.FieldOwner("kyma-sync-context")
)
