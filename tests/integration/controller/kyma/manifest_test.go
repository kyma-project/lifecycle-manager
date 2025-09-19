package kyma_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	machineryaml "k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/cpi"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/localblob"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg/componentmapping"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrEmptyModuleTemplateData = errors.New("module template spec.data is empty")
	ErrVersionMismatch         = errors.New("manifest spec.version mismatch with module template")
	ErrInvalidManifest         = errors.New("invalid ManifestResource")
)

var _ = Describe("Update Manifest CR", Ordered, func() {
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

		By("Update Module Template spec.data")
		moduleTemplateInCluster := &v1beta2.ModuleTemplate{}
		err := kcpClient.Get(ctx, client.ObjectKey{
			Name:      createModuleTemplateName(module),
			Namespace: kyma.GetNamespace(),
		}, moduleTemplateInCluster)
		Expect(err).ToNot(HaveOccurred())

		data := unstructured.Unstructured{}
		data.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   shared.OperatorGroup,
			Version: v1beta2.GroupVersion.Version,
			Kind:    "Sample",
		})
		data.Object["spec"] = map[string]interface{}{
			"initKey": "valueUpdated",
		}
		moduleTemplateInCluster.Spec.Data = &data

		Eventually(kcpClient.Update, Timeout, Interval).
			WithContext(ctx).
			WithArguments(moduleTemplateInCluster).
			Should(Succeed())

		By("CR updated with new value in spec.resource.spec")
		Eventually(expectManifestSpecDataEquals(kyma.Name, kyma.Namespace, "valueUpdated"), Timeout,
			Interval).Should(Succeed())
	})
})

func expectManifestSpecDataEquals(kymaName, kymaNamespace, value string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, kcpClient, kymaName, kymaNamespace)
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			if KCPModuleExistWithOverwrites(createdKyma, module) != value {
				return ErrSpecDataMismatch
			}
		}
		return nil
	}
}

var _ = Describe("Manifest.Spec is rendered correctly", Ordered, func() {
	kyma := NewTestKyma("kyma")
	module := NewTestModule("test-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("validate Manifest", func() {
		moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, module, kyma)
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
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
			Should(Succeed())

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
		moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, module, kyma)
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
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, moduleReferencedWithLabel, kyma).
				Should(Succeed())

			moduleTemplate, err := GetModuleTemplate(ctx, kcpClient, moduleReferencedWithLabel, kyma)
			Expect(err).ToNot(HaveOccurred())
			foundModuleName := moduleTemplate.Labels[shared.ModuleName]
			Expect(foundModuleName).To(Equal(moduleReferencedWithLabel.Name))
		})
	})

	Context("When operator is referenced by Namespace/Name", func() {
		It("cannot find the operator", func() {
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, moduleReferencedWithNamespacedName, kyma).
				Should(Equal(ErrNotFound))
		})
	})

	Context("When operator is referenced by FQDN", func() {
		It("cannot find the operator", func() {
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, moduleReferencedWithFQDN, kyma).
				Should(Equal(ErrNotFound))
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
	moduleTemplateDescriptor *types.Descriptor,
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
	moduleTemplateDescriptor *types.Descriptor,
) error {
	actualSourceName := manifestImageSpec.Name
	expectedSourceName := moduleTemplateDescriptor.Name

	if actualSourceName != expectedSourceName {
		return fmt.Errorf("Invalid SourceName: %s, expected: %s", actualSourceName, expectedSourceName)
	}
	return nil
}

func validateManifestSpecInstallSourceRef(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *types.Descriptor,
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

func extractInstallImageSpec(installInfo v1beta2.InstallInfo) *v1beta2.ImageSpec {
	var installImageSpec *v1beta2.ImageSpec
	err := machineryaml.Unmarshal(installInfo.Source.Raw, &installImageSpec)
	Expect(err).ToNot(HaveOccurred())
	return installImageSpec
}

func validateManifestSpecInstallSourceRepo(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *types.Descriptor,
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

// expectManifest is a generic Manifest assertion function.
func expectManifestFor(kyma *v1beta2.Kyma) func(func(*v1beta2.Manifest) error) func() error {
	return func(validationFn func(*v1beta2.Manifest) error) func() error {
		return func() error {
			// ensure manifest is refreshed each time the function is invoked 
			// for "Eventually" assertion to work correctly.
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
