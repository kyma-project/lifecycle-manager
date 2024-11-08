package kyma_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/cpi"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/localblob"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg/componentmapping"
	"ocm.software/ocm/api/utils/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	// see: PROJECT_ROOT/config/samples/component-integration-installed/operator_v1beta2_moduletemplate_kcp-module.yaml
	testModuleTemplateRawManifestLayerDigest = "sha256:" +
		"1735cfa45bf07b63427c8e11717278f8847e352a66af7633611db902386d18ed"

	// corresponds to the template-operator version: https://github.com/kyma-project/template-operator/commit/fc1cf2b4
	updatedModuleTemplateRawManifestLayerDigest = "sha256:" +
		"5aea8016459572585a57780c0aa348b5306bfa2cb4df7aa6d8b74e215b15e5dd"

	// registry: europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/template-operator:v3.1.0
	updatedModuleTemplateVersion = "v3.1.0"
)

var (
	ErrEmptyModuleTemplateData = errors.New("module template spec.data is empty")
	ErrVersionMismatch         = errors.New("manifest spec.version mismatch with module template")
	ErrInvalidManifest         = errors.New("invalid ManifestResource")
)

var _ = Describe("Manifest.Spec.Remote in default mode", Ordered, func() {
	kyma := NewTestKyma("kyma")

	module := NewTestModule("test-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("expect Manifest.Spec.Remote=false", func() {
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
			Should(BeFalse())
	})
})

var _ = Describe("Update Manifest CR", Ordered, func() {
	const updateRepositoryURL = "registry.docker.io/kyma-project"
	kyma := NewTestKyma("kyma-test-update")
	module := NewTestModule("test-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("Manifest CR should be updated after module template changed", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ManifestExists, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name).
				Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateManifestState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name,
					shared.StateReady).
				Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(KymaIsInState, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
			Should(Succeed())

		By("Update Module Template spec")
		var moduleTemplateFromFile v1beta2.ModuleTemplate
		builder.ReadComponentDescriptorFromFile("operator_v1beta2_moduletemplate_kcp-module_updated.yaml",
			&moduleTemplateFromFile)

		moduleTemplateInCluster := &v1beta2.ModuleTemplate{}
		err := kcpClient.Get(ctx, client.ObjectKey{
			Name:      createModuleTemplateName(module),
			Namespace: kyma.GetNamespace(),
		}, moduleTemplateInCluster)
		Expect(err).ToNot(HaveOccurred())

		moduleTemplateInCluster.Spec = moduleTemplateFromFile.Spec

		Eventually(kcpClient.Update, Timeout, Interval).
			WithContext(ctx).
			WithArguments(moduleTemplateInCluster).
			Should(Succeed())

		By("CR updated with new value in spec.resource.spec")
		Eventually(expectManifestSpecDataEquals(kyma.Name, "valueUpdated"), Timeout, Interval).Should(Succeed())

		By("Manifest is updated with new value in spec.install.source")
		{
			hasDummyRepositoryURL := func(manifest *v1beta2.Manifest) error {
				manifestImageSpec := extractInstallImageSpec(manifest.Spec.Install)
				if !strings.HasPrefix(manifestImageSpec.Repo, updateRepositoryURL) {
					return fmt.Errorf("Invalid manifest spec.install.repo: %s, expected prefix: %s",
						manifestImageSpec.Repo, updateRepositoryURL)
				}
				return nil
			}

			expectManifest := expectManifestFor(kyma)
			Eventually(expectManifest(hasDummyRepositoryURL), Timeout, Interval).Should(Succeed())
		}
	})
})

var _ = Describe("Manifest.Spec is rendered correctly", Ordered, func() {
	kyma := NewTestKyma("kyma")
	module := NewTestModule("test-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("validate Manifest", func() {
		moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, module, kyma.Spec.Channel, ControlPlaneNamespace)
		Expect(err).NotTo(HaveOccurred())

		expectManifest := expectManifestFor(kyma)

		By("checking Spec.Install")
		hasValidSpecInstall := func(manifest *v1beta2.Manifest) error {
			moduleTemplateDescriptor, err := descriptorProvider.GetDescriptor(moduleTemplate)
			if err != nil {
				return err
			}

			return validateManifestSpecInstallSource(extractInstallImageSpec(manifest.Spec.Install),
				moduleTemplateDescriptor)
		}
		Eventually(expectManifest(hasValidSpecInstall), Timeout, Interval).Should(Succeed())

		By("checking Spec.Resource")
		hasValidSpecResource := func(manifest *v1beta2.Manifest) error {
			return validateManifestSpecResource(manifest.Spec.Resource, moduleTemplate.Spec.Data)
		}
		Eventually(expectManifest(hasValidSpecResource), Timeout, Interval).Should(Succeed())

		By("checking Spec.Version")
		hasValidSpecVersion := func(manifest *v1beta2.Manifest) error {
			moduleTemplateDescriptor, err := descriptorProvider.GetDescriptor(moduleTemplate)
			if err != nil {
				return err
			}

			if manifest.Spec.Version != moduleTemplateDescriptor.Version {
				return ErrVersionMismatch
			}
			return nil
		}
		Eventually(expectManifest(hasValidSpecVersion), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Manifest.Spec is reset after manual update", Ordered, func() {
	const updateRepositoryURL = "registry.docker.io/kyma-project/component-descriptors"

	kyma := NewTestKyma("kyma")
	module := NewTestModule("test-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("update Manifest", func() {
		// await for the manifest to be created
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
			Should(BeFalse())

		manifest, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
		Expect(err).ToNot(HaveOccurred())

		manifestImageSpec := extractInstallImageSpec(manifest.Spec.Install)
		manifestImageSpec.Repo = updateRepositoryURL
		manifest.Spec.Version = "v1.7.0" // required to allow for SSA of manifest

		// is there a simpler way to update manifest.Spec.Install?
		updatedBytes, err := json.Marshal(manifestImageSpec)
		Expect(err).ToNot(HaveOccurred())
		manifest.Spec.Install.Source.Raw = updatedBytes

		err = kcpClient.Update(ctx, manifest)
		Expect(err).ToNot(HaveOccurred())
	})

	It("validate Manifest", func() {
		moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, module, kyma.Spec.Channel, ControlPlaneNamespace)
		Expect(err).NotTo(HaveOccurred())

		expectManifest := expectManifestFor(kyma)

		By("checking Spec.Install")
		hasValidSpecInstall := func(manifest *v1beta2.Manifest) error {
			moduleTemplateDescriptor, err := descriptorProvider.GetDescriptor(moduleTemplate)
			if err != nil {
				return err
			}

			return validateManifestSpecInstallSource(extractInstallImageSpec(manifest.Spec.Install),
				moduleTemplateDescriptor)
		}
		Eventually(expectManifest(hasValidSpecInstall), Timeout, Interval).Should(Succeed())

		By("checking Spec.Resource")
		hasValidSpecResource := func(manifest *v1beta2.Manifest) error {
			return validateManifestSpecResource(manifest.Spec.Resource, moduleTemplate.Spec.Data)
		}
		Eventually(expectManifest(hasValidSpecResource), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Update Module Template Version", Ordered, func() {
	kyma := NewTestKyma("kyma")
	module := NewTestModule("test-module", v1beta2.DefaultChannel)

	kyma.Spec.Modules = append(kyma.Spec.Modules, module)

	RegisterDefaultLifecycleForKyma(kyma)

	It("Manifest CR should be updated after module template version has changed", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ManifestExists, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name).
				Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateManifestState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name,
					shared.StateReady).
				Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(KymaIsInState, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
			Should(Succeed())

		By("Manifest spec.install.source.ref corresponds to Module Template resources[].access.digest")
		{
			manifest := expectManifestFor(kyma)
			hasInitialSourceRef := validateManifestSpecInstallSourceRefValue(testModuleTemplateRawManifestLayerDigest)

			Expect(manifest(hasInitialSourceRef)()).Should(Succeed())
		}

		By("Update Module Template version and raw-manifest layer digest")
		{
			newVersionAndLayerDigest := updateModuleTemplateVersion
			updatedVersionAndLayerDigest := validateModuleTemplateVersionUpdated
			updateModuleTemplateWith := funWrap(updateKCPModuleTemplate(module, kyma.Spec.Channel))
			validateModuleTemplateWith := funWrap(validateKCPModuleTemplate(module, kyma.Spec.Channel))

			updateModuleTemplateVersionAndLayerDigest := updateModuleTemplateWith(newVersionAndLayerDigest)
			validateVersionAndLayerDigestAreUpdated := validateModuleTemplateWith(updatedVersionAndLayerDigest)

			ensureModuleTemplateUpdate := series(
				updateModuleTemplateVersionAndLayerDigest,
				validateVersionAndLayerDigestAreUpdated,
			)

			Eventually(ensureModuleTemplateUpdate, Timeout, Interval*2).Should(Succeed())
		}

		By("Manifest is updated with new value in spec.install.source.ref")
		{
			expectManifest := expectManifestFor(kyma)
			hasUpdatedSourceRef := validateManifestSpecInstallSourceRefValue(updatedModuleTemplateRawManifestLayerDigest)

			Eventually(expectManifest(hasUpdatedSourceRef), Timeout, Interval*2).Should(Succeed())
		}
	})
})

var _ = Describe("Test Reconciliation Skip label for Manifest", Ordered, func() {
	kyma := NewTestKyma("kyma")
	module := NewTestModule("skip-reconciliation-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)

	RegisterDefaultLifecycleForKyma(kyma)
	It("Given a Manifest CR exists", func() {
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
			Should(Succeed())
	})

	It("When a skip label is added to Manifest", func() {
		Eventually(SetSkipLabelToManifest, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, true).
			Should(Succeed())
	})

	It("Then Skip label always exists in Manifest CR", func() {
		Consistently(SkipLabelExistsInManifest, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
			Should(BeTrue())
	})
})

var _ = Describe("Modules can only be referenced via module name", Ordered, func() {
	kyma := NewTestKyma("random-kyma")

	moduleReferencedWithLabel := NewTestModule("random-module", v1beta2.DefaultChannel)
	moduleReferencedWithNamespacedName := NewTestModule(
		v1beta2.DefaultChannel+shared.Separator+"random-module", v1beta2.DefaultChannel)
	moduleReferencedWithFQDN := NewTestModuleWithFixName("kyma-project.io/module/"+"random-module",
		v1beta2.DefaultChannel, "")
	kyma.Spec.Modules = append(kyma.Spec.Modules, moduleReferencedWithLabel)
	RegisterDefaultLifecycleForKyma(kyma)

	Context("When operator is referenced just by the label name", func() {
		It("returns the expected operator", func() {
			moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, moduleReferencedWithLabel, kyma.Spec.Channel,
				ControlPlaneNamespace)
			Expect(err).ToNot(HaveOccurred())

			foundModuleName := moduleTemplate.Labels[shared.ModuleName]
			Expect(foundModuleName).To(Equal(moduleReferencedWithLabel.Name))
		})
	})

	Context("When operator is referenced by Namespace/Name", func() {
		It("cannot find the operator", func() {
			_, err := GetModuleTemplate(ctx, kcpClient, moduleReferencedWithNamespacedName, kyma.Spec.Channel,
				ControlPlaneNamespace)
			Expect(err.Error()).Should(ContainSubstring(templatelookup.ErrNoTemplatesInListResult.Error()))
		})
	})

	Context("When operator is referenced by FQDN", func() {
		It("cannot find the operator", func() {
			_, err := GetModuleTemplate(ctx, kcpClient, moduleReferencedWithFQDN, kyma.Spec.Channel,
				ControlPlaneNamespace)
			Expect(err.Error()).Should(ContainSubstring(templatelookup.ErrNoTemplatesInListResult.Error()))
		})
	})
})

func findRawManifestResource(reslist []compdesc.Resource) *compdesc.Resource {
	for _, r := range reslist {
		if r.Name == string(v1beta2.RawManifestLayer) {
			return &r
		}
	}

	return nil
}

func validateManifestSpecInstallSource(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *v1beta2.Descriptor,
) error {
	if err := validateManifestSpecInstallSourceName(manifestImageSpec, moduleTemplateDescriptor); err != nil {
		return err
	}

	if err := validateManifestSpecInstallSourceRef(manifestImageSpec, moduleTemplateDescriptor); err != nil {
		return err
	}

	if err := validateManifestSpecInstallSourceRepo(manifestImageSpec, moduleTemplateDescriptor); err != nil {
		return err
	}

	return validateManifestSpecInstallSourceType(manifestImageSpec)
}

func validateManifestSpecInstallSourceName(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *v1beta2.Descriptor,
) error {
	actualSourceName := manifestImageSpec.Name
	expectedSourceName := moduleTemplateDescriptor.Name

	if actualSourceName != expectedSourceName {
		return fmt.Errorf("Invalid SourceName: %s, expected: %s", actualSourceName, expectedSourceName)
	}
	return nil
}

func validateManifestSpecInstallSourceRef(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *v1beta2.Descriptor,
) error {
	actualSourceRef := manifestImageSpec.Ref

	moduleTemplateResource := findRawManifestResource(moduleTemplateDescriptor.Resources)
	aspec, err := ocm.DefaultContext().AccessSpecForSpec(moduleTemplateResource.Access)
	if err != nil {
		return err
	}
	concreteAccessSpec, ok := aspec.(*localblob.AccessSpec)
	if !ok {
		return fmt.Errorf("Unexpected Resource Access Type: %T", aspec)
	}

	expectedSourceRef := concreteAccessSpec.LocalReference

	if actualSourceRef != expectedSourceRef {
		return fmt.Errorf("invalid manifest spec.install.source.ref: %s, expected: %s",
			actualSourceRef, expectedSourceRef)
	}

	return nil
}

func validateManifestSpecInstallSourceRefValue(expectedSourceRef string) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		manifestImageSpec := extractInstallImageSpec(manifest.Spec.Install)
		actualSourceRef := manifestImageSpec.Ref

		if actualSourceRef != expectedSourceRef {
			return fmt.Errorf("Invalid manifest spec.install.source.ref: %s, expected: %s",
				actualSourceRef, expectedSourceRef)
		}

		return nil
	}
}

func validateManifestSpecInstallSourceRepo(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *v1beta2.Descriptor,
) error {
	actualSourceRepo := manifestImageSpec.Repo

	unstructuredRepo := moduleTemplateDescriptor.GetEffectiveRepositoryContext()
	typedRepo, err := cpi.DefaultContext().RepositoryTypes().Convert(unstructuredRepo)
	if err != nil {
		return fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
	}
	concreteRepo, typeOk := typedRepo.(*genericocireg.RepositorySpec)
	if !typeOk {
		return fmt.Errorf("Unexpected Repository Type: %T", typedRepo)
	}

	repositoryBaseURL := concreteRepo.Name()
	if concreteRepo.SubPath != "" {
		repositoryBaseURL = concreteRepo.Name() + "/" + concreteRepo.SubPath
	}
	expectedSourceRepo := repositoryBaseURL + "/" + componentmapping.ComponentDescriptorNamespace

	if actualSourceRepo != expectedSourceRepo {
		return fmt.Errorf("Invalid SourceRepo: %s, expected: %s", actualSourceRepo, expectedSourceRepo)
	}

	return nil
}

func validateManifestSpecInstallSourceType(manifestImageSpec *v1beta2.ImageSpec) error {
	actualSourceType := string(manifestImageSpec.Type)
	expectedSourceType := string(v1beta2.OciRefType) // no corresponding value in the ModuleTemplate?

	if actualSourceType != expectedSourceType {
		return fmt.Errorf("Invalid SourceType: %s, expected: %s", actualSourceType, expectedSourceType)
	}
	return nil
}

func validateManifestSpecResource(manifestResource, moduleTemplateData *unstructured.Unstructured) error {
	if moduleTemplateData == nil {
		return ErrEmptyModuleTemplateData
	}
	actualManifestResource := manifestResource
	expectedManifestResource := moduleTemplateData.DeepCopy()
	expectedManifestResource.
		SetNamespace(flags.DefaultRemoteSyncNamespace) // the namespace is set in the "actual" object
	expectedManifestResource.SetName(actualManifestResource.GetName())
	if !reflect.DeepEqual(actualManifestResource, expectedManifestResource) {
		actualJSON, err := json.MarshalIndent(actualManifestResource, "", "  ")
		if err != nil {
			return err
		}
		expectedJSON, err := json.MarshalIndent(expectedManifestResource, "", "  ")
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: \nActual:\n%s\nExpected:\n%s", ErrInvalidManifest, actualJSON,
			expectedJSON)
	}
	return nil
}

// getKCPModuleTemplate is a generic ModuleTemplate validation function.
func validateKCPModuleTemplate(module v1beta2.Module, kymaChannel string) func(moduleTemplateFn) error {
	return func(validateFunc moduleTemplateFn) error {
		moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, module, kymaChannel, ControlPlaneNamespace)
		if err != nil {
			return err
		}

		err = validateFunc(moduleTemplate)
		if err != nil {
			return err
		}

		return nil
	}
}

// updateKCPModuleTemplate is a generic ModuleTemplate update function.
func updateKCPModuleTemplate(module v1beta2.Module, kymaChannel string) func(moduleTemplateFn) error {
	return func(updateFunc moduleTemplateFn) error {
		moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, module, kymaChannel, ControlPlaneNamespace)
		if err != nil {
			return err
		}

		err = updateFunc(moduleTemplate)
		if err != nil {
			return err
		}

		return kcpClient.Update(ctx, moduleTemplate)
	}
}

// expectManifest is a generic Manifest assertion function.
func expectManifestFor(kyma *v1beta2.Kyma) func(func(*v1beta2.Manifest) error) func() error {
	return func(validationFn func(*v1beta2.Manifest) error) func() error {
		return func() error {
			// ensure manifest is refreshed each time the function is invoked for "Eventually" assertion to work correctly.
			manifest, err := GetManifest(ctx, kcpClient,
				kyma.GetName(), kyma.GetNamespace(),
				kyma.Spec.Modules[0].Name)
			if err != nil {
				return err
			}
			return validationFn(manifest)
		}
	}
}

func updateComponentVersion(descriptor *v1beta2.Descriptor) {
	descriptor.ComponentSpec.Version = updatedModuleTemplateVersion
}

func updateComponentResources(descriptor *v1beta2.Descriptor) {
	resources := descriptor.ComponentSpec.Resources
	for i := range resources {
		res := &resources[i]
		res.Version = updatedModuleTemplateVersion

		if res.Name == string(v1beta2.RawManifestLayer) {
			object, ok := res.Access.(*runtime.UnstructuredVersionedTypedObject)
			Expect(ok).To(BeTrue())
			object.Object["digest"] = updatedModuleTemplateRawManifestLayerDigest
		}
	}
}

func updateComponentSources(descriptor *v1beta2.Descriptor) {
	sources := descriptor.ComponentSpec.Sources
	for i := range sources {
		src := &sources[i]
		src.Version = updatedModuleTemplateVersion
	}
}

func updateModuleTemplateVersion(moduleTemplate *v1beta2.ModuleTemplate) error {
	descriptor, err := descriptorProvider.GetDescriptor(moduleTemplate)
	if err != nil {
		return err
	}
	updateComponentVersion(descriptor)
	updateComponentResources(descriptor)
	updateComponentSources(descriptor)

	newDescriptorRaw, err := compdesc.Encode(descriptor.ComponentDescriptor, compdesc.DefaultJSONCodec)
	Expect(err).ToNot(HaveOccurred())
	moduleTemplate.Spec.Descriptor.Raw = newDescriptorRaw

	return nil
}

func validateModuleTemplateVersionUpdated(moduleTemplate *v1beta2.ModuleTemplate) error {
	descriptor, err := descriptorProvider.GetDescriptor(moduleTemplate)
	if err != nil {
		return err
	}
	expectedVersion := updatedModuleTemplateVersion

	if descriptor.Version != expectedVersion {
		return fmt.Errorf("Invalid descriptor version: %s, expected: %s", descriptor.Version, expectedVersion)
	}

	for _, res := range descriptor.ComponentSpec.Resources {
		if res.Version != expectedVersion {
			return fmt.Errorf("Invalid resource version: %s, expected: %s", res.Version, expectedVersion)
		}

		if res.Name == string(v1beta2.RawManifestLayer) {
			object, ok := res.Access.(*runtime.UnstructuredVersionedTypedObject)
			Expect(ok).To(BeTrue())
			if object.Object["digest"] != updatedModuleTemplateRawManifestLayerDigest {
				return fmt.Errorf("Invalid digest: %s, expected: %s",
					object.Object["digest"], updatedModuleTemplateRawManifestLayerDigest)
			}
		}
	}

	for _, source := range descriptor.ComponentSpec.Sources {
		if source.Version != updatedModuleTemplateVersion {
			return fmt.Errorf("Invalid source version: %s, expected: %s", source.Version, updatedModuleTemplateVersion)
		}
	}

	return nil
}

type moduleTemplateFn = func(*v1beta2.ModuleTemplate) error

// funWrap wraps a function return value into a parameterless function: "error" becomes "func() error".
func funWrap(inputFn func(moduleTemplateFn) error) func(moduleTemplateFn) func() error {
	res := func(actual moduleTemplateFn) func() error {
		return func() error {
			err := inputFn(actual)
			return err
		}
	}
	return res
}

// series wraps a list of simple parameterless functions into a single one.
func series(fns ...func() error) func() error {
	return func() error {
		var err error
		for i := range fns {
			err = fns[i]()
			if err != nil {
				return err
			}
		}
		return nil
	}
}
