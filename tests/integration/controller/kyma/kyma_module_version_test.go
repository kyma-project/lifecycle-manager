package kyma_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Given invalid module version which is rejected by CRD validation rules", func() {
	DescribeTable(
		"Test enable module", func(givenCondition func() error) {
			Skip("Version attribute is disabled for now on the CRD level")
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
		},

		Entry(
			"invalid semantic version",
			givenKymaWithInvalidModuleVersion("20240101"),
		),
		Entry(
			"invalid semantic version",
			givenKymaWithInvalidModuleVersion("1.0.0.abc"),
		),
	)
})

func givenKymaWithInvalidModuleVersion(version string) func() error {
	return func() error {
		kyma := NewTestKyma("kyma")
		kyma.Spec.Channel = v1beta2.DefaultChannel
		module := NewTestModuleWithChannelVersion("test", v1beta2.DefaultChannel, version)
		kyma.Spec.Modules = append(
			kyma.Spec.Modules, module)
		err := kcpClient.Create(ctx, kyma)
		return ignoreInvalidError(err)
	}
}
