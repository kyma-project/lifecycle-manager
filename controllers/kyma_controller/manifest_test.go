package kyma_controller_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/controllers"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-component-model/ocm/pkg/contexts/oci/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localociblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg/componentmapping"
)

var _ = Describe("Manifest.Spec.Remote in default mode", Ordered, func() {
	kyma := NewTestKyma("kyma")

	module := v1beta2.Module{
		ControllerName: "manifest",
		Name:           NewUniqModuleName(),
		Channel:        v1beta2.DefaultChannel,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("expect Manifest.Spec.Remote=false", func() {
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma, module).
			Should(Equal(false))
	})
})

var _ = Describe("Update Manifest CR", Ordered, func() {
	const updateRepositoryURL = "registry.docker.io/kyma-project"
	kyma := NewTestKyma("kyma-test-update")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta2.Module{
			ControllerName: "manifest",
			Name:           "skr-module-update",
			Channel:        v1beta2.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Manifest CR should be updated after module template changed", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ManifestExists, Timeout, Interval).WithArguments(
				ctx, kyma, activeModule, controlPlaneClient).Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateManifestState, Timeout, Interval).
				WithArguments(ctx, controlPlaneClient, kyma, activeModule, v1beta2.StateReady).Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(GetKymaState, Timeout, Interval).
			WithArguments(kyma.GetName()).
			Should(BeEquivalentTo(string(v1beta2.StateReady)))

		By("Update Module Template spec.data.spec field")
		valueUpdated := "valueUpdated"
		Eventually(updateKCPModuleTemplateSpecData(kyma.Name, valueUpdated), Timeout, Interval).Should(Succeed())

		By("CR updated with new value in spec.resource.spec")
		Eventually(expectManifestSpecDataEquals(kyma.Name, valueUpdated), Timeout, Interval).Should(Succeed())

		By("Update Module Template spec.descriptor.component values")
		{
			newComponentDescriptorRepositoryURL := func(moduleTemplate *v1beta2.ModuleTemplate) error {
				descriptor, err := moduleTemplate.GetDescriptor()
				if err != nil {
					return err
				}

				repositoryContext := descriptor.GetEffectiveRepositoryContext().Object
				_, ok := repositoryContext["baseUrl"].(string)
				if !ok {
					Fail("Can't find \"baseUrl\" property in ModuleTemplate spec")
				}
				repositoryContext["baseUrl"] = updateRepositoryURL

				newDescriptorRaw, err := compdesc.Encode(descriptor.ComponentDescriptor, compdesc.DefaultJSONLCodec)
				Expect(err).ToNot(HaveOccurred())
				moduleTemplate.Spec.Descriptor.Raw = newDescriptorRaw

				return nil
			}
			updateKCPModuleTemplateWith := updateKCPModuleTemplate("skr-module-update", "default")

			Eventually(updateKCPModuleTemplateWith(newComponentDescriptorRepositoryURL), Timeout, Interval).Should(Succeed())
		}

		By("Manifest is updated with new value in spec.install.source")
		{
			hasDummyRepositoryURL := func(manifest *v1beta2.Manifest) error {
				manifestImageSpec := extractInstallImageSpec(manifest.Spec.Install)
				if !strings.HasPrefix(manifestImageSpec.Repo, updateRepositoryURL) {
					return fmt.Errorf("Invalid manifest spec.install.repo: %s, expected prefix: %s", //nolint:goerr113
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

	module := v1beta2.Module{
		ControllerName: "manifest",
		Name:           NewUniqModuleName(),
		Channel:        v1beta2.DefaultChannel,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("validate Manifest", func() {
		moduleTemplate, err := GetModuleTemplate(ctx, controlPlaneClient, module.Name, "default")
		Expect(err).NotTo(HaveOccurred())

		expectManifest := expectManifestFor(kyma)

		By("checking Spec.Install")
		hasValidSpecInstall := func(manifest *v1beta2.Manifest) error {
			moduleTemplateDescriptor, err := moduleTemplate.GetDescriptor()
			if err != nil {
				return err
			}

			return validateManifestSpecInstallSource(extractInstallImageSpec(manifest.Spec.Install), moduleTemplateDescriptor)
		}
		Eventually(expectManifest(hasValidSpecInstall), Timeout, Interval).Should(Succeed())

		By("checking Spec.Resource")
		hasValidSpecResource := func(manifest *v1beta2.Manifest) error {
			return validateManifestSpecResource(manifest.Spec.Resource, &moduleTemplate.Spec.Data)
		}
		Eventually(expectManifest(hasValidSpecResource), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Manifest.Spec is reset after manual update", Ordered, func() {
	const updateRepositoryURL = "registry.docker.io/kyma-project/component-descriptors"

	kyma := NewTestKyma("kyma")

	module := v1beta2.Module{
		ControllerName: "manifest",
		Name:           NewUniqModuleName(),
		Channel:        v1beta2.DefaultChannel,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKyma(kyma)

	It("update Manifest", func() {
		// await for the manifest to be created
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma, module).
			Should(Equal(false))

		manifest, err := GetManifest(ctx, controlPlaneClient, kyma, module)
		Expect(err).ToNot(HaveOccurred())

		manifestImageSpec := extractInstallImageSpec(manifest.Spec.Install)
		manifestImageSpec.Repo = updateRepositoryURL

		// is there a simpler way to update manifest.Spec.Install?
		updatedBytes, err := json.Marshal(manifestImageSpec)
		Expect(err).ToNot(HaveOccurred())
		manifest.Spec.Install.Source.Raw = updatedBytes

		err = controlPlaneClient.Update(ctx, manifest)
		Expect(err).ToNot(HaveOccurred())
	})

	It("validate Manifest", func() {
		moduleTemplate, err := GetModuleTemplate(ctx, controlPlaneClient, module.Name, "default")
		Expect(err).NotTo(HaveOccurred())

		expectManifest := expectManifestFor(kyma)

		By("checking Spec.Install")
		hasValidSpecInstall := func(manifest *v1beta2.Manifest) error {
			moduleTemplateDescriptor, err := moduleTemplate.GetDescriptor()
			if err != nil {
				return err
			}

			return validateManifestSpecInstallSource(extractInstallImageSpec(manifest.Spec.Install), moduleTemplateDescriptor)
		}
		Eventually(expectManifest(hasValidSpecInstall), Timeout, Interval).Should(Succeed())

		By("checking Spec.Resource")
		hasValidSpecResource := func(manifest *v1beta2.Manifest) error {
			return validateManifestSpecResource(manifest.Spec.Resource, &moduleTemplate.Spec.Data)
		}
		Eventually(expectManifest(hasValidSpecResource), Timeout, Interval).Should(Succeed())
	})
})

func findRawManifestResource(reslist []compdesc.Resource) *compdesc.Resource {
	for _, r := range reslist {
		if r.Name == "raw-manifest" {
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

//nolint:goerr113
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

//nolint:goerr113
func validateManifestSpecInstallSourceRef(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *v1beta2.Descriptor,
) error {
	actualSourceRef := manifestImageSpec.Ref

	moduleTemplateResource := findRawManifestResource(moduleTemplateDescriptor.Resources)
	aspec, err := ocm.DefaultContext().AccessSpecForSpec(moduleTemplateResource.Access)
	if err != nil {
		return err
	}
	concreteAccessSpec, ok := aspec.(*localociblob.AccessSpec)
	if !ok {
		return fmt.Errorf("Unexpected Resource Access Type: %T", aspec)
	}

	expectedSourceRef := string(concreteAccessSpec.Digest)

	if actualSourceRef != expectedSourceRef {
		return fmt.Errorf("Invalid SourceRef: %s, expected: %s", actualSourceRef, expectedSourceRef)
	}

	return nil
}

//nolint:goerr113
func validateManifestSpecInstallSourceRepo(manifestImageSpec *v1beta2.ImageSpec,
	moduleTemplateDescriptor *v1beta2.Descriptor,
) error {
	actualSourceRepo := manifestImageSpec.Repo

	unstructuredRepo := moduleTemplateDescriptor.GetEffectiveRepositoryContext()
	typedRepo, err := unstructuredRepo.Evaluate(cpi.DefaultContext().RepositoryTypes())
	if err != nil {
		return fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
	}
	concreteRepo, typeOk := typedRepo.(*genericocireg.RepositorySpec)
	if !typeOk {
		return fmt.Errorf("Unexpected Repository Type: %T", typedRepo)
	}

	ociRepoSpec, typeOk := concreteRepo.RepositorySpec.(*ocireg.RepositorySpec)
	if !typeOk {
		return fmt.Errorf("Unexpected Repository Spec Type: %T", concreteRepo.RepositorySpec)
	}

	repositoryBaseURL := ociRepoSpec.BaseURL
	expectedSourceRepo := repositoryBaseURL + "/" + componentmapping.ComponentDescriptorNamespace

	if actualSourceRepo != expectedSourceRepo {
		return fmt.Errorf("Invalid SourceRepo: %s, expected: %s", actualSourceRepo, expectedSourceRepo)
	}

	return nil
}

//nolint:goerr113
func validateManifestSpecInstallSourceType(manifestImageSpec *v1beta2.ImageSpec) error {
	actualSourceType := string(manifestImageSpec.Type)
	expectedSourceType := "string(v1beta2.OciRefType)" // no corresponding value in the ModuleTemplate?

	if actualSourceType != expectedSourceType {
		return fmt.Errorf("Invalid SourceType: %s, expected: %s", actualSourceType, expectedSourceType)
	}
	return nil
}

func validateManifestSpecResource(manifestResource, moduleTemplateData *unstructured.Unstructured) error {
	actualManifestResource := manifestResource
	expectedManifestResource := moduleTemplateData.DeepCopy()
	expectedManifestResource.
		SetNamespace(controllers.DefaultRemoteSyncNamespace) // the namespace is set in the "actual" object

	if !reflect.DeepEqual(actualManifestResource, expectedManifestResource) {
		actualJSON, err := json.MarshalIndent(actualManifestResource, "", "  ")
		if err != nil {
			return err
		}
		expectedJSON, err := json.MarshalIndent(expectedManifestResource, "", "  ")
		if err != nil {
			return err
		}
		return fmt.Errorf("Invalid ManifestResource.\nActual:\n%s\nExpected:\n%s", actualJSON, expectedJSON) //nolint:goerr113
	}
	return nil
}

// updateKCPModuleTemplate is a generic ModuleTemplate update function.
func updateKCPModuleTemplate(
	moduleName, moduleNamespace string,
) func(func(*v1beta2.ModuleTemplate) error) func() error {
	return func(updateFunc func(*v1beta2.ModuleTemplate) error) func() error {
		return func() error {
			moduleTemplate, err := GetModuleTemplate(ctx, controlPlaneClient, moduleName, moduleNamespace)
			if err != nil {
				return err
			}

			err = updateFunc(moduleTemplate)
			if err != nil {
				return err
			}

			return controlPlaneClient.Update(ctx, moduleTemplate)
		}
	}
}

// expectManifest is a generic Manifest assertion function.
func expectManifestFor(kyma *v1beta2.Kyma) func(func(*v1beta2.Manifest) error) func() error {
	return func(validationFn func(*v1beta2.Manifest) error) func() error {
		return func() error {
			// ensure manifest is refreshed each time the function is invoked for "Eventually" assertion to work correctly.
			manifest, err := GetManifest(ctx, controlPlaneClient, kyma, kyma.Spec.Modules[0])
			if err != nil {
				return err
			}
			return validationFn(manifest)
		}
	}
}
