package control_plane_test

import (
	"errors"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	compdesc2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	ErrNotContainsExpectedCondition  = errors.New("kyma CR not contains expected condition")
	ErrNotContainsExpectedAnnotation = errors.New("kyma CR not contains expected CRD annotation")
	ErrContainsUnexpectedAnnotation  = errors.New("kyma CR contains unexpected CRD annotation")
	ErrAnnotationNotUpdated          = errors.New("kyma CR annotation not updated")
	ErrRemoteTemplateLabelNotFound   = errors.New("manifest does not contain remote template label")
)

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma")

	remoteKyma := &v1beta2.Kyma{}

	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = controller.DefaultRemoteSyncNamespace
	var runtimeClient client.Client
	var runtimeEnv *envtest.Environment
	var err error
	moduleInSKR := NewTestModule("in-skr", v1beta2.DefaultChannel)
	moduleInKCP := NewTestModule("in-kcp", v1beta2.DefaultChannel)
	customModuleInSKR := NewTestModule("custom-in-skr", v1beta2.DefaultChannel)
	customModuleInSKR.RemoteModuleTemplateRef = customModuleInSKR.Name

	defaultCR := builder.NewSampleCRBuilder().Build()

	SKRTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName(moduleInSKR.Name).
		WithChannel(moduleInSKR.Channel).
		WithModuleCR(defaultCR).
		WithOCM(compdesc2.SchemaVersion).Build()
	KCPTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName(moduleInKCP.Name).
		WithChannel(moduleInKCP.Channel).
		WithModuleCR(defaultCR).
		WithOCM(compdesc2.SchemaVersion).Build()
	SKRCustomTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName(customModuleInSKR.Name).
		WithChannel(customModuleInSKR.Channel).
		WithOCM(compdesc2.SchemaVersion).Build()

	BeforeAll(func() {
		runtimeClient, runtimeEnv, err = NewSKRCluster(controlPlaneClient.Scheme())
		Expect(err).NotTo(HaveOccurred())
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		DeleteModuleTemplates(ctx, controlPlaneClient, kyma)
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma contains global channel")
		Eventually(kymaChannelMatch, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace(), kyma.Spec.Channel).
			Should(Succeed())
	})

	It("ModuleTemplates should be synchronized in both clusters", func() {
		By("Module Template created")
		Eventually(controlPlaneClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(KCPTemplate).
			Should(Succeed())
		Eventually(controlPlaneClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(SKRTemplate).
			Should(Succeed())
		By("ModuleTemplate exists in KCP cluster")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, moduleInKCP, kyma.Spec.Channel).
			Should(Succeed())
		By("ModuleTemplate exists in SKR cluster")
		Eventually(ModuleTemplateExists, Timeout, Interval).WithArguments(ctx, runtimeClient, moduleInKCP,
			kyma.Spec.Channel).Should(Succeed())

		By("No module synced to remote Kyma")
		Eventually(NotContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("Remote Module Catalog created")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, runtimeClient, moduleInSKR, kyma.Spec.Channel).
			Should(Succeed())
		Eventually(containsModuleTemplateCondition, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), controller.DefaultRemoteSyncNamespace).
			Should(Succeed())
		Eventually(containsModuleTemplateCondition, Timeout, Interval).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma contains correct conditions for Modules and ModuleTemplates")
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(runtimeClient, v1beta2.ConditionTypeModules, string(v1beta2.ConditionReason),
				metav1.ConditionTrue, remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(runtimeClient, v1beta2.ConditionTypeModuleCatalog, string(v1beta2.ConditionReason),
				metav1.ConditionTrue, remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma should contain Watcher labels and annotations")
		Eventually(watcherLabelsAnnotationsExist, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma, kyma, remoteKyma.GetNamespace()).
			Should(Succeed())
	})

	It("Enable module in SKR Kyma CR", func() {
		By("add module to remote Kyma")
		Eventually(EnableModule, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace(), moduleInSKR).
			Should(Succeed())

		By("SKR module not sync back to KCP Kyma")
		Consistently(NotContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name).
			Should(Succeed())

		By("Manifest CR created in KCP")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name).
			Should(Succeed())
		By("KCP Manifest CR becomes ready")
		Eventually(UpdateManifestState, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name, shared.StateReady).
			Should(Succeed())

		By("ModuleTemplate descriptor should be saved in cache")
		Expect(DescriptorExistsInCache(SKRTemplate)).Should(BeTrue())

		By("Remote Kyma contains correct conditions for Modules")
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(runtimeClient, v1beta2.ConditionTypeModules, string(v1beta2.ConditionReason), metav1.ConditionTrue,
				remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())
	})

	It("Synced Module Template should get reset after changed", func() {
		By("Update SKR Module Template spec.data.spec field")
		Eventually(UpdateModuleTemplateSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, moduleInSKR, InitSpecKey, "valueUpdated", kyma.Spec.Channel).
			Should(Succeed())

		By("Expect SKR Module Template spec.data.spec field get reset")
		Eventually(expectModuleTemplateSpecGetReset, 2*Timeout, Interval).
			WithArguments(runtimeClient,
				moduleInSKR, kyma.Spec.Channel).
			Should(Succeed())
	})

	It("Remote SKR Kyma get regenerated after it gets deleted", func() {
		By("Delete SKR Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma).Should(Succeed())

		By("Expect SKR Kyma get recreated with no deletionTimestamp")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.GetName(), controller.DefaultRemoteSyncNamespace).
			Should(Succeed())
	})

	It("Enable Custom ModuleTemplate in SKR", func() {
		By("Create SKRCustomTemplate in SKR")
		SKRCustomTemplate.Namespace = kyma.Namespace
		Eventually(runtimeClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(SKRCustomTemplate).
			Should(Succeed())

		By("add module to remote Kyma")
		Eventually(EnableModule, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace(), customModuleInSKR).
			Should(Succeed())
	})

	It("Should not sync the SKRCustomTemplate in KCP and keep it only in SKR", func() {
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, runtimeClient, customModuleInSKR, kyma.Spec.Channel).
			Should(Succeed())
		Consistently(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, customModuleInSKR, kyma.Spec.Channel).
			Should(MatchError(channel.ErrNoTemplatesInListResult))
	})

	It("SKRCustomTemplate descriptor should not be saved in cache", func() {
		Expect(DescriptorExistsInCache(SKRCustomTemplate)).Should(BeFalse())
	})

	It("Should reconcile Manifest in KCP using remote SKRCustomTemplate", func() {
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), customModuleInSKR.Name).
			Should(Succeed())
	})

	It("Manifest should contain remoteModuleTemplate label", func() {
		Eventually(func() error {
			manifest, err := GetManifest(ctx, controlPlaneClient,
				kyma.GetName(), kyma.GetNamespace(),
				customModuleInSKR.Name)
			if err != nil {
				return err
			}

			if manifest.Labels[v1beta2.IsRemoteModuleTemplate] != v1beta2.EnableLabelValue {
				return ErrRemoteTemplateLabelNotFound
			}
			return nil
		}, Timeout, Interval).
			Should(Succeed())
	})

	It("Remote SKR Kyma get deleted when KCP Kyma get deleted", func() {
		By("Delete KCP Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())

		By("Expect SKR Kyma get deleted")
		Eventually(KymaDeleted, Timeout, Interval).
			WithContext(ctx).
			WithArguments(remoteKyma.GetName(), controller.DefaultRemoteSyncNamespace, runtimeClient).
			Should(Succeed())

		By("Make sure SKR Kyma not recreated")
		Consistently(KymaDeleted, Timeout, Interval).
			WithContext(ctx).
			WithArguments(remoteKyma.GetName(), controller.DefaultRemoteSyncNamespace, runtimeClient).
			Should(Succeed())

		By("SKRCustomTemplate should still exists in SKR")
		Consistently(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, runtimeClient, customModuleInSKR, kyma.Spec.Channel).
			Should(Succeed())
	})

	AfterAll(func() {
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})

var _ = Describe("Kyma sync default module list into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma")
	moduleInKCP := NewTestModule("in-kcp", v1beta2.DefaultChannel)
	kyma.Spec.Modules = []v1beta2.Module{{Name: moduleInKCP.Name, Channel: moduleInKCP.Channel}}

	var runtimeClient client.Client
	var runtimeEnv *envtest.Environment
	var err error
	remoteKyma := &v1beta2.Kyma{}
	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = controller.DefaultRemoteSyncNamespace

	BeforeAll(func() {
		runtimeClient, runtimeEnv, err = NewSKRCluster(controlPlaneClient.Scheme())
		Expect(err).NotTo(HaveOccurred())
	})
	registerControlPlaneLifecycleForKyma(kyma)

	It("Kyma CR default module list should be copied to remote Kyma", func() {
		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace).
			Should(Succeed())

		By("Remote Kyma contains default module")
		Eventually(ContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being created")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(Succeed())
	})

	It("Delete default module from remote Kyma", func() {
		By("Delete default module from remote Kyma")
		Eventually(DisableModule, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being deleted")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(MatchError(ErrNotFound))
	})

	It("Default module list should be recreated if remote Kyma gets deleted", func() {
		By("Delete remote Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma).Should(Succeed())
		By("Remote Kyma contains default module")
		Eventually(ContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being created")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(Succeed())
	})
	AfterAll(func() {
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})

var _ = Describe("CRDs sync to SKR and annotations updated in KCP kyma", Ordered, func() {
	kyma := NewTestKyma("kyma-test-crd-update")
	moduleInKcp := NewTestModule("in-kcp", v1beta2.DefaultChannel)
	kyma.Spec.Modules = []v1beta2.Module{moduleInKcp}

	remoteKyma := &v1beta2.Kyma{}

	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = controller.DefaultRemoteSyncNamespace
	var runtimeClient client.Client
	var runtimeEnv *envtest.Environment
	var err error
	BeforeAll(func() {
		runtimeClient, runtimeEnv, err = NewSKRCluster(controlPlaneClient.Scheme())
		Expect(err).NotTo(HaveOccurred())
	})
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

			if kcpKyma.Annotations["kyma-skr-crd-generation"] != strconv.FormatInt(skrKymaCrd.Generation, 10) {
				return ErrAnnotationNotUpdated
			}
			if kcpKyma.Annotations["kyma-kcp-crd-generation"] != strconv.FormatInt(kcpKymaCrd.Generation, 10) {
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

	AfterAll(func() {
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})
