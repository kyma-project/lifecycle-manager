package kcp_test

import (
	"errors"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var ErrNotContainsExpectedCondition = errors.New("kyma CR not contains expected condition")

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	var err error
	kyma := NewTestKyma("kyma-1")
	skrKyma := NewSKRKyma()
	moduleInSKR := NewTestModule("skrmodule", v1beta2.DefaultChannel)
	moduleInKCP := NewTestModule("kcpmodule", v1beta2.DefaultChannel)
	defaultCR := builder.NewModuleCRBuilder().WithSpec(InitSpecKey, InitSpecValue).Build()
	moduleInSKROCMName := FullOCMName(moduleInSKR.Name)
	moduleInSKROCM := MustNewComponentId(moduleInSKROCMName, moduleVersion)
	moduleInKCPOCMName := FullOCMName(moduleInKCP.Name)

	TemplateForSKREnabledModule := builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName(moduleInSKR.Name, moduleVersion)).
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleInSKR.Name).
		WithVersion(moduleVersion).
		WithModuleCR(defaultCR).
		Build()

	TemplateForKCPEnabledModule := builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName(moduleInKCP.Name, moduleVersion)).
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleInKCP.Name).
		WithVersion(moduleVersion).
		WithModuleCR(defaultCR).
		Build()
	var skrClient client.Client
	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		DeleteModuleTemplates(ctx, kcpClient, kyma)
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
	})

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma contains global channel")
		Eventually(kymaChannelMatch, Timeout, Interval).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace(), kyma.Spec.Channel).
			Should(Succeed())
	})

	It("KCP ModuleTemplates should be created", func() {
		Eventually(CreateModuleTemplate, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, TemplateForKCPEnabledModule).
			Should(Succeed())
		Eventually(CreateModuleTemplate, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, TemplateForSKREnabledModule).
			Should(Succeed())
	})

	It("ModuleReleaseMeta should be created in KCP", func() {
		err := registerDescriptor(moduleInSKROCMName, moduleVersion)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(configureKCPModuleReleaseMeta, Timeout, Interval).WithArguments(moduleInSKR.Name).Should(Succeed())
		err = registerDescriptor(moduleInKCPOCMName, moduleVersion)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(configureKCPModuleReleaseMeta, Timeout, Interval).WithArguments(moduleInKCP.Name).Should(Succeed())
	})

	It("ModuleTemplates should be synchronized in both clusters", func() {
		By("ModuleTemplate exists in KCP cluster")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, kcpClient, moduleInKCP, kyma).
			Should(Succeed())
		By("ModuleTemplate exists in SKR cluster")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, skrClient, moduleInKCP, skrKyma).Should(Succeed())

		By("No module synced to remote Kyma")
		Eventually(NotContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("Remote Module Catalog created")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, skrClient, moduleInSKR, skrKyma).
			Should(Succeed())
		Eventually(containsModuleTemplateCondition, Timeout, Interval).
			WithArguments(skrClient, skrKyma.GetName(), flags.DefaultRemoteSyncNamespace).
			Should(Succeed())
		Eventually(containsModuleTemplateCondition, Timeout, Interval).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma contains correct conditions for Modules and ModuleTemplates")
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(skrClient, v1beta2.ConditionTypeModules, string(v1beta2.ConditionReason),
				apimetav1.ConditionTrue, skrKyma.GetName(), skrKyma.GetNamespace()).
			Should(Succeed())
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(skrClient, v1beta2.ConditionTypeModuleCatalog, string(v1beta2.ConditionReason),
				apimetav1.ConditionTrue, skrKyma.GetName(), skrKyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma should contain Watcher labels and annotations")
		Eventually(watcherLabelsAnnotationsExist, Timeout, Interval).
			WithArguments(skrClient, skrKyma, kyma, skrKyma.GetNamespace()).
			Should(Succeed())
	})

	It("Enable module in SKR Kyma CR", func() {
		By("add module to remote Kyma")
		Eventually(EnableModule, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace(), moduleInSKR).
			Should(Succeed())

		By("SKR module not sync back to KCP Kyma")
		Consistently(NotContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name).
			Should(Succeed())

		By("Manifest CR created in KCP")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name).
			Should(Succeed())
		By("KCP Manifest CR becomes ready")
		Eventually(UpdateManifestState, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name, shared.StateReady).
			Should(Succeed())

		By("component descriptor should be saved in cache")
		Expect(isDescriptorCached(*moduleInSKROCM)).Should(BeTrue())

		By("Remote Kyma contains correct conditions for Modules")
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(skrClient, v1beta2.ConditionTypeModules, string(v1beta2.ConditionReason),
				apimetav1.ConditionTrue,
				skrKyma.GetName(), skrKyma.GetNamespace()).
			Should(Succeed())
	})

	It("Synced Module Template should get reset after changed", func() {
		By("Update SKR Module Template spec.data.spec field")
		Eventually(UpdateModuleTemplateSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, moduleInSKR, InitSpecKey, "valueUpdated", skrKyma).
			Should(Succeed())

		By("Expect SKR Module Template spec.data.spec field get reset")
		Eventually(expectModuleTemplateSpecGetReset, 2*Timeout, Interval).
			WithArguments(skrClient, moduleInSKR, skrKyma).
			Should(Succeed())
	})

	It("Remote SKR Kyma get regenerated after it gets deleted", func() {
		By("Delete SKR Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma).Should(Succeed())

		By("Expect SKR Kyma get recreated with no deletionTimestamp")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), flags.DefaultRemoteSyncNamespace).
			Should(Succeed())
	})

	It("Remote SKR Kyma get deleted when KCP Kyma get deleted", func() {
		By("Delete KCP Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())

		By("Expect SKR Kyma get deleted")
		Eventually(KymaDeleted, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrKyma.GetName(), flags.DefaultRemoteSyncNamespace, skrClient).
			Should(Succeed())

		By("Make sure SKR Kyma not recreated")
		Consistently(KymaDeleted, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrKyma.GetName(), flags.DefaultRemoteSyncNamespace, skrClient).
			Should(Succeed())
	})
})

// isDescriptorCached checks if the descriptor is in the cache.
// It temporarily stops the underlying DescriptorService to ensure the cache is used
// instead of DescriptorService lookup.
func isDescriptorCached(ocmId ocmidentity.ComponentId) bool {
	descProviderService.Stop()
	defer descProviderService.Resume()
	result, err := descriptorProvider.GetDescriptor(ocmId)
	return err == nil && result != nil
}

var _ = Describe("Kyma sync default module list into Remote Cluster", Ordered, func() {
	var skrClient client.Client
	var err error

	kyma := NewTestKyma("kyma-2")
	skrKyma := NewSKRKyma()
	moduleInKCP := NewTestModule("kcpmodule", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, moduleInKCP)

	moduleInKCPOCMName := FullOCMName(moduleInKCP.Name)

	templateForModuleInKCP := builder.NewModuleTemplateBuilder().
		WithName(fmt.Sprintf("%s-%s", moduleInKCP.Name, moduleVersion)).
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleInKCP.Name).
		WithVersion(moduleVersion).
		Build()

	registerControlPlaneLifecycleForKyma(kyma)
	BeforeAll(func() {
		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())
	})

	It("ModuleTemplate for default module should be created in KCP", func() {
		Eventually(CreateModuleTemplate, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, templateForModuleInKCP).
			Should(Succeed())
	})

	It("ModuleReleaseMeta should be created in KCP", func() {
		err := registerDescriptor(moduleInKCPOCMName, moduleVersion)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(configureKCPModuleReleaseMeta, Timeout, Interval).
			WithArguments(moduleInKCP.Name).Should(Succeed())
	})

	It("Kyma CR default module list should be copied to remote Kyma", func() {
		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.Name, skrKyma.Namespace).
			Should(Succeed())

		By("Remote Kyma contains default module")
		Eventually(ContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.Name, skrKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being created")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(Succeed())
	})

	It("Delete default module from remote Kyma", func() {
		manifestInCluster, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(),
			moduleInKCP.Name)
		Expect(err).Should(Succeed())
		By("Delete default module from remote Kyma")
		Eventually(DisableModule, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.Name, skrKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being deleted")
		Eventually(ManifestExistsByMetadata, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, manifestInCluster.Namespace, manifestInCluster.Name).
			Should(Equal(ErrNotFound))
	})

	It("Default module list should be recreated if remote Kyma gets deleted", func() {
		By("Delete remote Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma).Should(Succeed())
		By("Remote Kyma contains default module")
		Eventually(ContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.Name, skrKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being created")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(Succeed())
	})
})

var _ = Describe("CRDs sync to SKR and annotations updated in KCP kyma", Ordered, func() {
	kyma := NewTestKyma("kyma-test-crd-update")
	moduleInKCP := NewTestModuleWithChannelVersion("module-inkcp", v1beta2.DefaultChannel, "0.1.0")
	moduleTemplateName := v1beta2.CreateModuleTemplateName(moduleInKCP.Name, moduleInKCP.Version)

	moduleReleaseMetaInKCP := builder.NewModuleReleaseMetaBuilder().
		WithName("modulereleasemeta-inkcp").
		WithModuleName(moduleInKCP.Name).
		WithSingleModuleChannelAndVersions(v1beta2.DefaultChannel, "0.1.0").
		Build()
	kyma.Spec.Modules = []v1beta2.Module{
		{
			Name:    moduleInKCP.Name,
			Channel: moduleInKCP.Channel,
		},
	}
	var skrClient client.Client
	var err error
	BeforeAll(func() {
		template := builder.NewModuleTemplateBuilder().
			WithName(moduleTemplateName).
			WithNamespace(ControlPlaneNamespace).
			WithModuleName(moduleInKCP.Name).
			WithVersion(moduleInKCP.Version).
			Build()
		Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(template).
			Should(Succeed())
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())

		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())

		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, moduleReleaseMetaInKCP).Should(Succeed())
	})
	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		DeleteModuleTemplates(ctx, kcpClient, kyma)
	})

	It("Kyma CRDsSync condition should be set to true", func() {
		Eventually(func() bool {
			kcpKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return false
			}
			return kcpKyma.ContainsCondition(v1beta2.ConditionTypeCRDsSync, apimetav1.ConditionTrue)
		}, Timeout, Interval).Should(BeTrue())
	})

	It("Kyma CRD should sync to SKR", func() {
		var kcpKymaCrd *apiextensionsv1.CustomResourceDefinition
		var skrKymaCrd *apiextensionsv1.CustomResourceDefinition
		var skrModuleReleaseMetaCrd *apiextensionsv1.CustomResourceDefinition
		var kcpModuleReleaseMetaCrd *apiextensionsv1.CustomResourceDefinition

		By("Update KCP Kyma CRD")
		Eventually(func() string {
			var err error
			kcpKymaCrd, err = updateKymaCRD(kcpClient)
			if err != nil {
				return ""
			}

			return getCrdSpec(kcpKymaCrd).Properties["channel"].Description
		}, Timeout, Interval).Should(Equal("test change"))

		By("SKR Kyma CRD should be updated")
		Eventually(func() *apiextensionsv1.CustomResourceValidation {
			var err error
			skrKymaCrd, err = fetchCrd(skrClient, shared.KymaKind)
			if err != nil {
				return nil
			}

			return skrKymaCrd.Spec.Versions[0].Schema
		}, Timeout, Interval).Should(Equal(kcpKymaCrd.Spec.Versions[0].Schema))

		By("Update ModuleReleaseMeta CRD")
		Eventually(func() string {
			var err error
			kcpModuleReleaseMetaCrd, err = updateModuleReleaseMetaCRD(kcpClient)
			if err != nil {
				return ""
			}

			return getCrdSpec(kcpModuleReleaseMetaCrd).Properties["channels"].Description
		}, Timeout, Interval).Should(Equal("test change"))

		By("SKR ModuleReleaseMeta CRD should be updated")
		Eventually(func() *apiextensionsv1.CustomResourceValidation {
			var err error
			skrModuleReleaseMetaCrd, err = fetchCrd(skrClient, shared.ModuleReleaseMetaKind)
			if err != nil {
				return nil
			}

			return skrModuleReleaseMetaCrd.Spec.Versions[0].Schema
		}, Timeout, Interval).Should(Equal(kcpModuleReleaseMetaCrd.Spec.Versions[0].Schema))
	})
})
