package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Blocking Module Deletion With Multiple Module CRs - Ignore Policy", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	module.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		testModuleDeletionBlocking(
			kyma,
			&module,
			[]types.NamespacedName{
				{Name: "sample-cr-1", Namespace: RemoteNamespace},
				{Name: "sample-cr-2", Namespace: RemoteNamespace},
			},
			false,
		)
	})
})
