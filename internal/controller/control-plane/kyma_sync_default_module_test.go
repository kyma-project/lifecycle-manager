package control_plane_test

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	compdesc2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var _ = Describe("CRDs sync to SKR and annotations updated in KCP kyma", Ordered, func() {
	kyma := NewTestKyma("kyma-test-crd-update")
	moduleInKcp := NewTestModule("in-kcp", v1beta2.DefaultChannel)
	kyma.Spec.Modules = []v1beta2.Module{moduleInKcp}

	remoteKyma := &v1beta2.Kyma{}

	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = controller.DefaultRemoteSyncNamespace

	registerControlPlaneLifecycleForKyma(kyma)
	annotations := []string{
		"moduletemplate-skr-crd-generation",
		"moduletemplate-kcp-crd-generation",
		"kyma-skr-crd-generation",
		"kyma-kcp-crd-generation",
	}

	It("module template created", func() {
		template := builder.NewModuleTemplateBuilder().
			WithModuleName(moduleInKcp.Name).
			WithChannel(moduleInKcp.Channel).
			WithOCM(compdesc2.SchemaVersion).Build()
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, template).
			Should(Succeed())
	})

	It("CRDs generation annotation should exist in KCP kyma", func() {
		Eventually(func() error {
			kcpKyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}

			for _, annotation := range annotations {
				if _, ok := kcpKyma.Annotations[annotation]; !ok {
					return ErrNotContainsExpectedAnnotation
				}
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("CRDs generation annotation shouldn't exist in SKR kyma", func() {
		Eventually(func() error {
			skrKyma, err := GetKyma(ctx, runtimeClient, remoteKyma.GetName(), controller.DefaultRemoteSyncNamespace)
			if err != nil {
				return err
			}

			for _, annotation := range annotations {
				if _, ok := skrKyma.Annotations[annotation]; ok {
					return ErrContainsUnexpectedAnnotation
				}
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("Kyma CRD should sync to SKR and annotations get updated", func() {
		var kcpKymaCrd *v1.CustomResourceDefinition
		var skrKymaCrd *v1.CustomResourceDefinition
		By("Update KCP Kyma CRD")
		Eventually(func() string {
			var err error
			kcpKymaCrd, err = updateKymaCRD(controlPlaneClient)
			if err != nil {
				return ""
			}

			return getCrdSpec(kcpKymaCrd).Properties["channel"].Description
		}, Timeout, Interval).Should(Equal("test change"))

		By("SKR Kyma CRD should be updated")
		Eventually(func() *v1.CustomResourceValidation {
			var err error
			skrKymaCrd, err = fetchCrd(runtimeClient, v1beta2.KymaKind)
			if err != nil {
				return nil
			}

			return skrKymaCrd.Spec.Versions[0].Schema
		}, Timeout, Interval).Should(Equal(kcpKymaCrd.Spec.Versions[0].Schema))

		By("Kyma CR generation annotations should be updated")
		Eventually(func() error {
			kcpKyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}

			if kcpKyma.Annotations["kyma-skr-crd-generation"] != fmt.Sprint(skrKymaCrd.Generation) {
				return ErrAnnotationNotUpdated
			}
			if kcpKyma.Annotations["kyma-kcp-crd-generation"] != fmt.Sprint(skrKymaCrd.Generation) {
				return ErrAnnotationNotUpdated
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("Should regenerate Kyma CRD in SKR when deleted", func() {
		kymaCrd, err := fetchCrd(runtimeClient, v1beta2.KymaKind)
		Expect(err).NotTo(HaveOccurred())
		Eventually(runtimeClient.Delete, Timeout, Interval).
			WithArguments(ctx, kymaCrd).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func() error {
			_, err := fetchCrd(runtimeClient, v1beta2.KymaKind)
			return err
		}, Timeout, Interval).WithContext(ctx).Should(Not(HaveOccurred()))
	})

	It("Should regenerate ModuleTemplate CRD in SKR when deleted", func() {
		moduleTemplateCrd, err := fetchCrd(runtimeClient, v1beta2.ModuleTemplateKind)
		Expect(err).NotTo(HaveOccurred())
		Eventually(runtimeClient.Delete, Timeout, Interval).
			WithArguments(ctx, moduleTemplateCrd).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func() error {
			_, err := fetchCrd(runtimeClient, v1beta2.ModuleTemplateKind)
			return err
		}, Timeout, Interval).WithContext(ctx).Should(Not(HaveOccurred()))
	})
})
