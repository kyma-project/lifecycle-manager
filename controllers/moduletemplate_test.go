package controllers_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ErrManifestRemoteIsNotMatch = errors.New("Manifest.Spec.Remote is not match")

func expectManifestSpecRemoteMatched(kymaName string, remoteFlag bool) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			moduleInCluster, err := getModule(kymaName, module.Name)
			// Expect(err).ShouldNot(HaveOccurred())
			if err != nil {
				return err
			}
			manifestSpec := UnmarshalManifestSpec(moduleInCluster)
			if manifestSpec.Remote != remoteFlag {
				return ErrManifestRemoteIsNotMatch
			}
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func updateModuleTemplateTarget(kymaName string, target v1alpha1.Target) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			moduleTemplate, err := GetModuleTemplate(module.Name)
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
		kyma.Spec.Modules, v1alpha1.Module{
			ControllerName: "manifest",
			Name:           NewUniqModuleName(),
			Channel:        v1alpha1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	DescribeTable("Test ModuleTemplate.Spec.Target",
		func(givenCondition func() error, expectedBehavior func() error) {
			Skip("skip now")

			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry("When ModuleTemplate.Spec.Target not exist deployed, expect Manifest.Spec.remote=false",
			noCondition(),
			expectManifestSpecRemoteMatched(kyma.Name, true)),
		Entry("When update ModuleTemplate.Spec.Target=remote, expect Manifest.Spec.remote=true",
			updateModuleTemplateTarget(kyma.Name, v1alpha1.TargetRemote),
			expectManifestSpecRemoteMatched(kyma.Name, true)),
		Entry("When update ModuleTemplate.Spec.Target=control-plane, expect Manifest.Spec.remote=false",
			updateModuleTemplateTarget(kyma.Name, v1alpha1.TargetControlPlane),
			expectManifestSpecRemoteMatched(kyma.Name, false)),
	)
})
