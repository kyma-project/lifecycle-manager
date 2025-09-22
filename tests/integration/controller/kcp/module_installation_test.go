package kcp_test

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

// version is the same as configured for the ModuleTemplate by using "WithOCM()".
const moduleVersion = "1.1.1-e2e-test"

var _ = Describe("Module installation", func() {
	DescribeTable(
		"Verify module installation for different beta/internal configurations",
		func(moduleBeta, moduleInternal, kymaBeta, kymaInternal, shouldHaveInstallation bool) {
			kymaName := "installation-test-kyma-" + random.Name()
			moduleName := "installation-test-module-" + random.Name()

			Eventually(configureKCPKyma, Timeout, Interval).WithArguments(kymaName, kymaBeta,
				kymaInternal).Should(Succeed())
			Eventually(configureKCPModuleTemplates, Timeout, Interval).WithArguments(moduleName, moduleBeta,
				moduleInternal).Should(Succeed())
			Eventually(configureKCPModuleReleaseMeta, Timeout, Interval).WithArguments(moduleName).Should(Succeed())

			var skrClient client.Client
			var err error
			Eventually(func() error {
				skrClient, err = testSkrContextFactory.Get(types.NamespacedName{
					Name:      kymaName,
					Namespace: ControlPlaneNamespace,
				})
				return err
			}, Timeout, Interval).Should(Succeed())

			Eventually(configureSKRKyma, Timeout, Interval).WithArguments(moduleName,
				skrClient).Should(Succeed())

			if shouldHaveInstallation {
				Eventually(expectInstallation, Timeout, Interval).WithArguments(kymaName, moduleName).Should(Succeed())
			} else {
				// we use Consistently here as the installation may require multiple reconciliation runs to be installed
				// otherwise, we may get a false positive after the first reconciliation 
				// where no module is installed yet, but in a consecutive run it will be
				Consistently(expectNoInstallation, Timeout, Interval).WithArguments(kymaName,
					moduleName).Should(Succeed())
			}
		},
		Entry(
			"Given Module{Beta: false, Internal: false}; Kyma{Beta: false, Internal: false}; Expect Installation:true",
			false,
			false,
			false,
			false,
			true,
		),
		Entry(
			"Given Module{Beta: true, Internal: false}; Kyma{Beta: false, Internal: false}; Expect Installation: false",
			true,
			false,
			false,
			false,
			false,
		),
		Entry(
			"Given Module{Beta: false, Internal: true}; Kyma{Beta: false, Internal: false}; Expect Installation: false",
			false,
			true,
			false,
			false,
			false,
		),
		Entry(
			"Given Module{Beta: false, Internal: false}; Kyma{Beta: true, Internal: false}; Expect Installation:  true",
			false,
			false,
			true,
			false,
			true,
		),
		Entry(
			"Given Module{Beta: false, Internal: false}; Kyma{Beta: false, Internal: true}; Expect Installation:  true",
			false,
			false,
			false,
			true,
			true,
		),
		Entry(
			"Given Module{Beta: true, Internal: true}; Kyma{Beta: false, Internal: false}; Expect Installation:  false",
			true,
			true,
			false,
			false,
			false,
		),
		Entry(
			"Given Module{Beta: true, Internal: false}; Kyma{Beta: true, Internal: false}; Expect Installation:  true",
			true,
			false,
			true,
			false,
			true,
		),
		Entry(
			"Given Module{Beta: true, Internal: false}; Kyma{Beta: false, Internal: true}; Expect Installation:  false",
			true,
			false,
			false,
			true,
			false,
		),
		Entry(
			"Given Module{Beta: true, Internal: true}; Kyma{Beta: true, Internal: false}; Expect Installation:  false",
			true,
			true,
			true,
			false,
			false,
		),
		Entry(
			"Given Module{Beta: true, Internal: true}; Kyma{Beta: false, Internal: true}; Expect Installation:  false",
			true,
			true,
			false,
			true,
			false,
		),
		Entry("Given Module{Beta: true, Internal: true}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			true, true, true, true, true),
		Entry(
			"Given Module{Beta: false, Internal: true}; Kyma{Beta: true, Internal: false}; Expect Installation:  false",
			false,
			true,
			true,
			false,
			false,
		),
		Entry(
			"Given Module{Beta: false, Internal: true}; Kyma{Beta: false, Internal: true}; Expect Installation:  true",
			false,
			true,
			false,
			true,
			true,
		),
		Entry(
			"Given Module{Beta: false, Internal: false}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			false,
			false,
			true,
			true,
			true,
		),
		Entry("Given Module{Beta: false, Internal: true}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			false, true, true, true, true),
		Entry("Given Module{Beta: true, Internal: false}; Kyma{Beta: true, Internal: true}; Expect Installation:  true",
			true, false, true, true, true),
	)
})

func configureKCPKyma(kymaName string, beta, internal bool) error {
	kyma := builder.NewKymaBuilder().
		WithName(kymaName).
		WithNamespace(ControlPlaneNamespace).
		WithAnnotation(shared.SkrDomainAnnotation, "example.domain.com").
		WithLabel(shared.InstanceIDLabel, "test-instance").
		WithChannel(v1beta2.DefaultChannel).
		Build()

	if beta {
		kyma.Labels[shared.BetaLabel] = shared.EnableLabelValue
	}
	if internal {
		kyma.Labels[shared.InternalLabel] = shared.EnableLabelValue
	}

	Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
		WithArguments(kyma).
		Should(Succeed())

	return nil
}

func configureSKRKyma(moduleName string, skrClient *remote.SkrContext) error {
	kyma := v1beta2.Kyma{}
	err := skrClient.Get(context.Background(),
		types.NamespacedName{Name: shared.DefaultRemoteKymaName, Namespace: shared.DefaultRemoteNamespace}, &kyma)
	if err != nil {
		return err
	}

	kyma.Spec.Modules = append(kyma.Spec.Modules, NewTestModuleWithFixName(moduleName, v1beta2.DefaultChannel, ""))
	err = skrClient.Update(context.Background(), &kyma)
	if err != nil {
		return err
	}

	return nil
}

func configureKCPModuleTemplates(moduleName string, moduleBeta, moduleInternal bool) error {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithName(fmt.Sprintf("%s-%s", moduleName, moduleVersion)).
		WithModuleName(moduleName).
		WithVersion(moduleVersion).
		WithOCM(compdescv2.SchemaVersion).
		WithBeta(moduleBeta).
		WithInternal(moduleInternal).
		Build()

	Eventually(kcpClient.Create, Timeout, Interval).
		WithContext(ctx).
		WithArguments(moduleTemplate).
		Should(Succeed())

	return nil
}

func configureKCPModuleReleaseMeta(moduleName string) error {
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleName).
		WithSingleModuleChannelAndVersions(v1beta2.DefaultChannel, moduleVersion).
		Build()

	Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
		WithArguments(moduleReleaseMeta).
		Should(Succeed())

	return nil
}

func expectInstallation(kymaName, moduleName string) error {
	manifest, _ := GetManifest(ctx, kcpClient, kymaName, ControlPlaneNamespace, moduleName)
	if manifest != nil {
		return nil
	} else {
		return errors.New("expected manifest to be installed, but it was not")
	}
}

func expectNoInstallation(kymaName, moduleName string) error {
	manifest, _ := GetManifest(ctx, kcpClient, kymaName, ControlPlaneNamespace, moduleName)
	if manifest == nil {
		return nil
	} else {
		return errors.New("expected manifest to not be installed, but it was")
	}
}
