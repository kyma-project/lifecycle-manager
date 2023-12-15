package mandatory_test

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrWrongModulesStatus = errors.New("modules status not correct")
)

var _ = Describe("Mandatory Module Installation", Ordered, func() {

	Context("Given Kyma with no Module and one mandatory ModuleTemplate on Control-Plane", func() {
		kyma := NewTestKyma("no-module-kyma")
		registerControlPlaneLifecycleForKyma(kyma)

		It("Then Kyma CR should result in a ready state immediately as there are no modules", func() {
			Eventually(KymaIsInState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("And Kyma CR should contain empty status.modules", func() {
			Eventually(func() error {
				createdKyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
				if err != nil {
					return err
				}
				if len(createdKyma.Status.Modules) != 0 {
					return ErrWrongModulesStatus
				}
				return nil
			}, Timeout, Interval).
				Should(Succeed())
		})

		By("And Manifest CR for the Kyma should be created", func() {
			// TODO
		})

	})
})

func DeployMandatoryModuleTemplateWithName(ctx context.Context, kcpClient client.Client, name string) {
	template := builder.NewModuleTemplateBuilder().
		WithModuleName(name).
		WithChannel("mandatory").
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).Build()
	Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
		WithArguments(template).
		Should(Succeed())

}

func DeleteMandatoryModuleTemplateWithName(ctx context.Context, kcpClient client.Client, name string) {
	template := builder.NewModuleTemplateBuilder().
		WithModuleName(name).
		WithChannel("mandatory").
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).Build()
	Eventually(kcpClient.Delete, Timeout, Interval).WithContext(ctx).
		WithArguments(template).
		Should(Succeed())

}

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		DeployMandatoryModuleTemplateWithName(ctx, controlPlaneClient, "mandatory-module")
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		DeleteMandatoryModuleTemplateWithName(ctx, controlPlaneClient, "mandatory-module")
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})
}
