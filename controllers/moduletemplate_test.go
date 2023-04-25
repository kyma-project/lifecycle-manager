package controllers_test

import (
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrManifestRemoteIsNotMatch              = errors.New("Manifest.Spec.Remote is not match")
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
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, metav1.NamespaceDefault)
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			if SKRModuleExistWithOverwrites(createdKyma, module) != value {
				return ErrSpecDataMismatch
			}
		}
		return nil
	}
}

func expectManifestSpecRemoteMatched(kymaName string, remoteFlag bool) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, metav1.NamespaceDefault)
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			moduleInCluster, err := getModule(createdKyma, module)
			if err != nil {
				return err
			}
			if moduleInCluster.Spec.Remote != remoteFlag {
				return ErrManifestRemoteIsNotMatch
			}
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func expectManifestSpecNotContainsCredSecretSelector(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			moduleInCluster, err := getModule(createdKyma, module)
			if err != nil {
				return err
			}
			if moduleInCluster.Spec.Config.CredSecretSelector != nil {
				return ErrContainsUnexpectedCredSecretSelector
			}
			installImageSpec := extractInstallImageSpec(moduleInCluster.Spec.Install)

			if installImageSpec.CredSecretSelector != nil {
				return ErrContainsUnexpectedCredSecretSelector
			}
		}
		return nil
	}
}

func expectManifestSpecContainsCredSecretSelector(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			moduleInCluster, err := getModule(createdKyma, module)
			if err != nil {
				return err
			}
			var emptyImageSpec v1beta2.ImageSpec
			if moduleInCluster.Spec.Config != emptyImageSpec {
				if err := expectCredSecretSelectorCorrect(&moduleInCluster.Spec.Config); err != nil {
					return fmt.Errorf("config %v is invalid: %w", &moduleInCluster.Spec.Config, err)
				}
			}

			installImageSpec := extractInstallImageSpec(moduleInCluster.Spec.Install)
			if err := expectCredSecretSelectorCorrect(installImageSpec); err != nil {
				return fmt.Errorf("install %v is invalid: %w", installImageSpec, err)
			}
		}
		return nil
	}
}

func extractInstallImageSpec(installInfo v1beta2.InstallInfo) *v1beta2.ImageSpec {
	var installImageSpec *v1beta2.ImageSpec
	err := yaml.Unmarshal(installInfo.Source.Raw, &installImageSpec)
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

func updateModuleTemplateTarget(kymaName string, target v1beta2.Target) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			moduleTemplate, err := GetModuleTemplate(module.Name, controlPlaneClient, createdKyma, false)
			if err != nil {
				return err
			}
			moduleTemplate.Spec.Target = target
			err = controlPlaneClient.Update(ctx, moduleTemplate)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

var _ = Describe("Test ModuleTemplate CR", Ordered, func() {
	kyma := NewTestKyma("kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta2.Module{
			ControllerName: "manifest",
			Name:           NewUniqModuleName(),
			Channel:        v1beta2.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	DescribeTable("Test ModuleTemplate.Spec.Target",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry("When ModuleTemplate.Spec.Target not exist deployed, expect Manifest.Spec.remote=false",
			noCondition(),
			expectManifestSpecRemoteMatched(kyma.Name, false)),
		Entry("When update ModuleTemplate.Spec.Target=remote, expect Manifest.Spec.remote=true",
			updateModuleTemplateTarget(kyma.Name, v1beta2.TargetRemote),
			expectManifestSpecRemoteMatched(kyma.Name, true)),
		Entry("When update ModuleTemplate.Spec.Target=control-plane, expect Manifest.Spec.remote=false",
			updateModuleTemplateTarget(kyma.Name, v1beta2.TargetControlPlane),
			expectManifestSpecRemoteMatched(kyma.Name, false)),
	)
})

var _ = Describe("Test ModuleTemplate.Spec.descriptor not contains RegistryCred label", Ordered, func() {
	kyma := NewTestKyma("kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta2.Module{
			ControllerName: "manifest",
			Name:           NewUniqModuleName(),
			Channel:        v1beta2.DefaultChannel,
		})

	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

	It("expect Manifest.Spec.installs and Manifest.Spec.Config not contains credSecretSelector", func() {
		DeployModuleTemplates(ctx, controlPlaneClient, kyma, false)
		Eventually(expectManifestSpecNotContainsCredSecretSelector(kyma.Name), Timeout*2, Interval).Should(Succeed())
	})
})

var _ = Describe("Test ModuleTemplate.Spec.descriptor contains RegistryCred label", Ordered, func() {
	kyma := NewTestKyma("kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta2.Module{
			ControllerName: "manifest",
			Name:           NewUniqModuleName(),
			Channel:        v1beta2.DefaultChannel,
		})

	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

	It("expect Manifest.Spec.installs and Manifest.Spec.Config contains credSecretSelector", func() {
		DeployModuleTemplates(ctx, controlPlaneClient, kyma, true)
		Eventually(expectManifestSpecContainsCredSecretSelector(kyma.Name), Timeout*2, Interval).Should(Succeed())
	})
})
