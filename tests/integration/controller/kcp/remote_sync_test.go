package kcp_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrNotContainsExpectedCondition  = errors.New("kyma CR not contains expected condition")
	ErrNotContainsExpectedAnnotation = errors.New("kyma CR not contains expected CRD annotation")
	ErrContainsUnexpectedAnnotation  = errors.New("kyma CR contains unexpected CRD annotation")
	ErrAnnotationNotUpdated          = errors.New("kyma CR annotation not updated")
)

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-1")
	skrKyma := NewSKRKyma()
	moduleInSKR := NewTestModule("skr-module", v1beta2.DefaultChannel)
	moduleInKCP := NewTestModule("kcp-module", v1beta2.DefaultChannel)
	defaultCR := builder.NewModuleCRBuilder().WithSpec(InitSpecKey, InitSpecValue).Build()
	TemplateForSKREnabledModule := builder.NewModuleTemplateBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleInSKR.Name).
		WithChannel(moduleInSKR.Channel).
		WithModuleCR(defaultCR).
		WithOCM(compdescv2.SchemaVersion).Build()
	TemplateForKCPEnabledModule := builder.NewModuleTemplateBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleInKCP.Name).
		WithChannel(moduleInKCP.Channel).
		WithModuleCR(defaultCR).
		WithOCM(compdescv2.SchemaVersion).Build()
	var skrClient client.Client
	var err error
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

		By("ModuleTemplate descriptor should be saved in cache")
		Expect(IsDescriptorCached(TemplateForSKREnabledModule)).Should(BeTrue())

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

func IsDescriptorCached(template *v1beta2.ModuleTemplate) bool {
	key := descriptorProvider.GenerateDescriptorKey(template.Name, template.GetVersion())
	result := descriptorProvider.DescriptorCache.Get(key)
	return result != nil
}

var _ = Describe("Kyma sync default module list into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-2")
	moduleInKCP := NewTestModule("kcp-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, moduleInKCP)
	skrKyma := NewSKRKyma()
	var skrClient client.Client
	var err error
	registerControlPlaneLifecycleForKyma(kyma)
	BeforeAll(func() {
		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())
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
	moduleTemplateName := fmt.Sprintf("%s-%s", moduleInKCP.Name, "0.1.0")

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
	skrKyma := NewSKRKyma()
	var skrClient client.Client
	var err error
	BeforeAll(func() {
		template := builder.NewModuleTemplateBuilder().
			WithName(moduleInKCP.Name).
			WithNamespace(ControlPlaneNamespace).
			WithModuleName(moduleInKCP.Name).
			WithChannel(moduleInKCP.Channel).
			WithOCM(compdescv2.SchemaVersion).
			WithName(moduleTemplateName).Build()
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

	injectedAnnotations := []string{
		"modulereleasemeta-skr-crd-generation",
		"modulereleasemeta-kcp-crd-generation",
		"moduletemplate-skr-crd-generation",
		"moduletemplate-kcp-crd-generation",
		"kyma-skr-crd-generation",
		"kyma-kcp-crd-generation",
	}

	It("CRDs generation annotation should exist in KCP kyma", func() {
		Eventually(func() error {
			kcpKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}

			relevantKymaAnnotations := collections.Filter(slices.Collect(maps.Keys(kcpKyma.Annotations)),
				func(val string) bool {
					return strings.HasSuffix(val, "crd-generation")
				})
			if len(relevantKymaAnnotations) < len(injectedAnnotations) {
				return fmt.Errorf("%w: expected: %d, actual: %d", ErrNotContainsExpectedAnnotation,
					len(injectedAnnotations), len(relevantKymaAnnotations))
			}
			for _, expectedAnnotation := range injectedAnnotations {
				if _, ok := kcpKyma.Annotations[expectedAnnotation]; !ok {
					return fmt.Errorf("%w: %s is missing", ErrNotContainsExpectedAnnotation, expectedAnnotation)
				}
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("CRDs generation annotation shouldn't exist in SKR kyma", func() {
		Eventually(func() error {
			skrKyma, err := GetKyma(ctx, skrClient, skrKyma.GetName(), flags.DefaultRemoteSyncNamespace)
			if err != nil {
				return err
			}

			for _, unwantedAnnotation := range injectedAnnotations {
				if _, ok := skrKyma.Annotations[unwantedAnnotation]; ok {
					return fmt.Errorf("%w: %s is present but it should not", ErrContainsUnexpectedAnnotation,
						unwantedAnnotation)
				}
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("Kyma CRD should sync to SKR and annotations get updated", func() {
		var kcpKymaCrd *apiextensionsv1.CustomResourceDefinition
		var skrKymaCrd *apiextensionsv1.CustomResourceDefinition
		var skrModuleTemplateCrd *apiextensionsv1.CustomResourceDefinition
		var kcpModuleTemplateCrd *apiextensionsv1.CustomResourceDefinition
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

		By("Read ModuleTemplate CRDs")
		Eventually(func() error {
			var err error
			skrModuleTemplateCrd, err = fetchCrd(skrClient, shared.ModuleTemplateKind)
			if err != nil {
				return err
			}
			kcpModuleTemplateCrd, err = fetchCrd(kcpClient, shared.ModuleTemplateKind)
			if err != nil {
				return err
			}
			return nil
		}, Timeout, Interval).Should(Succeed())

		By("Kyma CR generation annotations should be updated")
		Eventually(func() error {
			kcpKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}

			if err = assertCrdGenerationAnnotations(kcpKyma, "kyma-skr-crd-generation", skrKymaCrd); err != nil {
				return err
			}
			if err = assertCrdGenerationAnnotations(kcpKyma, "kyma-kcp-crd-generation", kcpKymaCrd); err != nil {
				return err
			}
			if err = assertCrdGenerationAnnotations(kcpKyma, "moduletemplate-skr-crd-generation",
				skrModuleTemplateCrd); err != nil {
				return err
			}
			if err = assertCrdGenerationAnnotations(kcpKyma, "moduletemplate-kcp-crd-generation",
				kcpModuleTemplateCrd); err != nil {
				return err
			}
			if err = assertCrdGenerationAnnotations(kcpKyma, "modulereleasemeta-skr-crd-generation",
				skrModuleReleaseMetaCrd); err != nil {
				return err
			}
			if err = assertCrdGenerationAnnotations(kcpKyma, "modulereleasemeta-kcp-crd-generation",
				kcpModuleReleaseMetaCrd); err != nil {
				return err
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("Should regenerate Kyma CRD in SKR when deleted", func() {
		kymaCrd, err := fetchCrd(skrClient, shared.KymaKind)
		Expect(err).NotTo(HaveOccurred())
		Eventually(skrClient.Delete, Timeout, Interval).
			WithArguments(ctx, kymaCrd).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func() error {
			_, err := fetchCrd(skrClient, shared.KymaKind)
			return err
		}, Timeout, Interval).WithContext(ctx).Should(Not(HaveOccurred()))
	})

	It("Should regenerate ModuleTemplate CRD in SKR when deleted", func() {
		moduleTemplateCrd, err := fetchCrd(skrClient, shared.ModuleTemplateKind)
		Expect(err).NotTo(HaveOccurred())
		Eventually(skrClient.Delete, Timeout, Interval).
			WithArguments(ctx, moduleTemplateCrd).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func() error {
			_, err := fetchCrd(skrClient, shared.ModuleTemplateKind)
			return err
		}, Timeout, Interval).WithContext(ctx).Should(Not(HaveOccurred()))
	})
})

func assertCrdGenerationAnnotations(kcpKyma *v1beta2.Kyma, annotationName string,
	targetCrd *apiextensionsv1.CustomResourceDefinition,
) error {
	annotationValue := kcpKyma.Annotations[annotationName]
	targetCrdGeneration := strconv.FormatInt(targetCrd.Generation, 10)
	if annotationValue != targetCrdGeneration {
		return fmt.Errorf("%w: expected: %s, actual: %s", ErrAnnotationNotUpdated, targetCrdGeneration, annotationValue)
	}
	return nil
}
