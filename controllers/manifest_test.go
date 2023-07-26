package controllers_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
					return errors.New(fmt.Sprintf("Invalid manifest spec.install.repo: %s, expected prefix: %s", manifestImageSpec.Repo, updateRepositoryURL))
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
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma, module).
			Should(Equal(false))

		manifest, err := GetManifest(ctx, controlPlaneClient, kyma, module)
		Expect(err).To(BeNil())

		moduleTemplate, err := GetModuleTemplate(ctx, controlPlaneClient, module.Name, "default")
		Expect(err).To(BeNil())

		By("checking Spec.Install")
		validateManifestSpecInstall(manifest.Spec.Install, moduleTemplate)

		By("checking Spec.Resource")
		validateManifestSpecResource(manifest.Spec.Resource, &moduleTemplate.Spec.Data)
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
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma, module).
			Should(Equal(false))

		manifest, err := GetManifest(ctx, controlPlaneClient, kyma, module)
		Expect(err).ToNot(HaveOccurred())

		dumpToScreen(manifest.Spec.Install, "====")
		manifestImageSpec := extractInstallImageSpec(manifest.Spec.Install)
		manifestImageSpec.Repo = updateRepositoryURL

		updatedBytes, err := json.Marshal(manifestImageSpec)
		Expect(err).ToNot(HaveOccurred())
		manifest.Spec.Install.Source.Raw = updatedBytes

		dumpToScreen(manifest.Spec.Install, ">>>>")
		err = controlPlaneClient.Update(ctx, manifest)
		Expect(err).ToNot(HaveOccurred())
	})

	It("validate Manifest", func() {
		time.Sleep(10)
		manifest, err := GetManifest(ctx, controlPlaneClient, kyma, module)
		Expect(err).To(BeNil())

		moduleTemplate, err := GetModuleTemplate(ctx, controlPlaneClient, module.Name, "default")
		Expect(err).To(BeNil())

		By("checking Spec.Install")
		validateManifestSpecInstall(manifest.Spec.Install, moduleTemplate)

		By("checking Spec.Resource")
		validateManifestSpecResource(manifest.Spec.Resource, &moduleTemplate.Spec.Data)
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

func validateManifestSpecInstall(manifestInstall v1beta2.InstallInfo, moduleTemplate *v1beta2.ModuleTemplate) {

	var (
		manifestImageSpec        *v1beta2.ImageSpec
		moduleTemplateDescriptor *v1beta2.Descriptor
		err                      error
	)

	manifestImageSpec = extractInstallImageSpec(manifestInstall)
	moduleTemplateDescriptor, err = moduleTemplate.GetDescriptor()
	Expect(err).To(BeNil())

	//compares the actual manifest spec.install.source.name with the corresponding values from the ModuleTemplate
	compareManifestSourceName := func() {
		actualSourceName := manifestImageSpec.Name
		expectedSourceName := moduleTemplateDescriptor.Name

		Expect(actualSourceName).To(Equal(expectedSourceName))
	}

	//compares the actual manifest spec.install.source.ref with the corresponding values from the ModuleTemplate
	compareManifestSourceRef := func() {
		actualSourceRef := manifestImageSpec.Ref

		moduleTemplateResource := findRawManifestResource(moduleTemplateDescriptor.Resources)
		aspec, err := ocm.DefaultContext().AccessSpecForSpec(moduleTemplateResource.Access)
		concreteAccessSpec, ok := aspec.(*localociblob.AccessSpec)
		if !ok {
			err = errors.New(fmt.Sprintf("Unexpected Resource Access Type: %T", aspec))
			Expect(err).ToNot(HaveOccurred())
		}

		expectedSourceRef := string(concreteAccessSpec.Digest)

		Expect(actualSourceRef).To(Equal(expectedSourceRef))
	}

	//compares the actual manifest spec.install.source.repo with the corresponding values from the ModuleTemplate
	compareManifestSourceRepo := func() {
		actualSourceRepo := manifestImageSpec.Repo

		unstructuredRepo := moduleTemplateDescriptor.GetEffectiveRepositoryContext()
		typedRepo, err := unstructuredRepo.Evaluate(cpi.DefaultContext().RepositoryTypes())
		if err != nil {
			err = fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
			return
		}
		concreteRepo, ok := typedRepo.(*genericocireg.RepositorySpec)
		if !ok {
			err = errors.New(fmt.Sprintf("Unexpected Repository Type: %T", typedRepo))
			Expect(err).ToNot(HaveOccurred())
		}

		ociRepoSpec, ok := concreteRepo.RepositorySpec.(*ocireg.RepositorySpec)
		if !ok {
			err = errors.New(fmt.Sprintf("Unexpected Repository Spec Type: %T", concreteRepo.RepositorySpec))
			Expect(err).ToNot(HaveOccurred())
		}

		repositoryBaseURL := ociRepoSpec.BaseURL
		expectedSourceRepo := repositoryBaseURL + "/" + componentmapping.ComponentDescriptorNamespace

		Expect(actualSourceRepo).To(Equal(expectedSourceRepo))
	}

	//validates the actual manifest spec.install.source.type
	compareManifestSourceType := func() {
		actualSourceType := string(manifestImageSpec.Type)
		expectedSourceType := "oci-ref" //no corresponding value in the ModuleTemplate?

		Expect(actualSourceType).To(Equal(expectedSourceType))
	}

	compareManifestSourceName()
	compareManifestSourceRef()
	compareManifestSourceRepo()
	compareManifestSourceType()
}

func validateManifestSpecResource(manifestResource, moduleTemplateData *unstructured.Unstructured) {

	actualManifestResource := manifestResource
	expectedManifestResource := moduleTemplateData.DeepCopy()
	expectedManifestResource.SetNamespace(controllers.DefaultRemoteSyncNamespace)

	Expect(actualManifestResource).To(Equal(expectedManifestResource))
}

// updateKCPModuleTemplate is a generic ModuleTemplate update function
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

// expectManifest is a generic Manifest assertion function
func expectManifestFor(kyma *v1beta2.Kyma) func(func(*v1beta2.Manifest) error) func() error {

	return func(validationFn func(*v1beta2.Manifest) error) func() error {
		return func() error {
			manifest, err := GetManifest(ctx, controlPlaneClient, kyma, kyma.Spec.Modules[0])
			if err != nil {
				return err
			}
			return validationFn(manifest)
		}
	}
}

func dumpToScreen(val any, prefix string) {
	vSer, err := json.MarshalIndent(val, prefix, "  ")
	if err != nil {
		panic(fmt.Errorf("Error when dumping value: %#v: %w", val, err))
	}
	fmt.Println(string(vSer))
}
