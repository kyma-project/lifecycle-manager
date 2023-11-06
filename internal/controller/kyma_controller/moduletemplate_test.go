package kyma_controller_test

import (
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrContainsUnexpectedCredSecretSelector  = errors.New("contains unexpected credSecretSelector")
	ErrNotContainsExpectedCredSecretSelector = errors.New("not contains expected credSecretSelector")
)

const (
	//nolint:gosec
	credSecretLabel = "operator.kyma-project.io/oci-registry-cred"
	//nolint:gosec
	credSecretValue = "operator-regcred"
)

func expectManifestSpecDataEquals(kymaName, value string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, apimetav1.NamespaceDefault)
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

func expectManifestSpecNotContainsCredSecretSelector(kymaName, kymaNamespace string) error {
	kyma, err := GetKyma(ctx, controlPlaneClient, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	for _, module := range kyma.Spec.Modules {
		moduleInCluster, err := GetManifest(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
		if err != nil {
			return err
		}
		installImageSpec := extractInstallImageSpec(moduleInCluster.Spec.Install)

		if installImageSpec.CredSecretSelector != nil {
			return ErrContainsUnexpectedCredSecretSelector
		}
	}
	return nil
}

func expectManifestSpecContainsCredSecretSelector(kymaName, kymaNamespace string) error {
	kyma, err := GetKyma(ctx, controlPlaneClient, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	for _, module := range kyma.Spec.Modules {
		moduleInCluster, err := GetManifest(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
		if err != nil {
			return err
		}

		installImageSpec := extractInstallImageSpec(moduleInCluster.Spec.Install)
		if err := expectCredSecretSelectorCorrect(installImageSpec); err != nil {
			return fmt.Errorf("install %v is invalid: %w", installImageSpec, err)
		}
	}
	return nil
}

func extractInstallImageSpec(installInfo v1beta2.InstallInfo) *v1beta2.ImageSpec {
	var installImageSpec *v1beta2.ImageSpec
	err := machineryaml.Unmarshal(installInfo.Source.Raw, &installImageSpec)
	Expect(err).ToNot(HaveOccurred())
	return installImageSpec
}

func expectCredSecretSelectorCorrect(installImageSpec *v1beta2.ImageSpec) error {
	if installImageSpec.CredSecretSelector == nil {
		return fmt.Errorf("image spec %v does not contain credSecretSelector: %w",
			installImageSpec, ErrNotContainsExpectedCredSecretSelector)
	}

	value, found := installImageSpec.CredSecretSelector.MatchLabels[credSecretLabel]
	Expect(found).To(BeTrue())
	Expect(value).To(Equal(credSecretValue))
	return nil
}

var _ = Describe("ModuleTemplate.Spec.descriptor not contains RegistryCred label", Ordered, func() {
	kyma := NewTestKyma("kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, NewTestModule("test-module", v1beta2.DefaultChannel))

	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

	It("expect Manifest.Spec.installs not contains credSecretSelector", func() {
		DeployModuleTemplates(ctx, controlPlaneClient, kyma)
		Eventually(expectManifestSpecNotContainsCredSecretSelector, Timeout, Interval).
			WithArguments(kyma.Name, kyma.Namespace).Should(Succeed())
	})
})

var _ = Describe("ModuleTemplate.Spec.descriptor contains RegistryCred label", Ordered, func() {
	kyma := NewTestKyma("kyma")
	module := NewTestModule("test-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)

	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

	It("expect Manifest.Spec.installs contains credSecretSelector", func() {
		template := builder.NewModuleTemplateBuilder().
			WithModuleName(module.Name).
			WithChannel(module.Channel).
			WithOCMPrivateRepo().Build()
		Eventually(controlPlaneClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(template).
			Should(Succeed())
		Eventually(expectManifestSpecContainsCredSecretSelector, Timeout, Interval).
			WithArguments(kyma.Name, kyma.Namespace).Should(Succeed())
	})
})
