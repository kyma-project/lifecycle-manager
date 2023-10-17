package control_plane_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	compdesc2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrContainsUnexpectedModules     = errors.New("kyma CR contains unexpected modules")
	ErrNotContainsExpectedModules    = errors.New("kyma CR not contains expected modules")
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

	moduleInSKR := NewTestModule("in-skr", v1beta2.DefaultChannel)
	moduleInKCP := NewTestModule("in-kcp", v1beta2.DefaultChannel)
	customModuleInSKR := NewTestModule("custom-in-skr", v1beta2.DefaultChannel)
	customModuleInSKR.RemoteModuleTemplateRef = customModuleInSKR.Name

	defaultCR := builder.NewModuleCRBuilder().WithSpec(InitSpecKey, InitSpecValue).Build()

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
		Eventually(notContainsModuleInSpec, Timeout, Interval).
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
		Eventually(addModuleToKyma, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace(), moduleInSKR).
			Should(Succeed())

		By("SKR module not sync back to KCP Kyma")
		Consistently(notContainsModuleInSpec, Timeout, Interval).
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
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name, v1beta2.StateReady).
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
		Eventually(addModuleToKyma, Timeout, Interval).
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
