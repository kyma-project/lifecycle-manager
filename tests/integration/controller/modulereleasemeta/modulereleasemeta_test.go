package modulereleasemeta_test

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	thenNoErrors               = " then expect no validation errors"
	thenExpectAValidationError = " then expect a validation error"
)

var _ = Describe("ModuleReleaseMeta validation", Ordered, func() {
	DescribeTable("Validate XValidation rule for mandatory vs channels",
		func(
			setupFunc func() *v1beta2.ModuleReleaseMeta,
			shouldSucceed bool,
		) {
			meta := setupFunc()

			err := kcpClient.Create(ctx, meta)
			if shouldSucceed {
				Expect(err).NotTo(HaveOccurred())
				err = kcpClient.Delete(ctx, meta)
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exactly one of 'mandatory' or 'channels' must be specified"))
			}
		},
		Entry("when only mandatory is specified,"+
			thenNoErrors,
			func() *v1beta2.ModuleReleaseMeta {
				return builder.NewModuleReleaseMetaBuilder().
					WithName("test-mandatory").
					WithNamespace(ControlPlaneNamespace).
					WithModuleName("test-module").
					WithMandatory("1.0.0").
					Build()
			},
			true,
		),
		Entry("when only channels is specified,"+
			thenNoErrors,
			func() *v1beta2.ModuleReleaseMeta {
				return builder.NewModuleReleaseMetaBuilder().
					WithName("test-channels").
					WithNamespace(ControlPlaneNamespace).
					WithModuleName("test-module").
					WithSingleModuleChannelAndVersions("stable", "1.0.0").
					Build()
			},
			true,
		),
		Entry("when both mandatory and channels are specified,"+
			thenExpectAValidationError,
			func() *v1beta2.ModuleReleaseMeta {
				meta := builder.NewModuleReleaseMetaBuilder().
					WithName("test-both").
					WithNamespace(ControlPlaneNamespace).
					WithModuleName("test-module").
					WithSingleModuleChannelAndVersions("stable", "1.0.0").
					WithMandatory("1.0.0").
					Build()
				return meta
			},
			false,
		),
		Entry("when neither mandatory nor channels are specified,"+
			thenExpectAValidationError,
			func() *v1beta2.ModuleReleaseMeta {
				meta := builder.NewModuleReleaseMetaBuilder().
					WithName("test-neither").
					WithNamespace(ControlPlaneNamespace).
					WithModuleName("test-module").
					Build()
				meta.Spec.Channels = nil
				meta.Spec.Mandatory = nil
				return meta
			},
			false,
		),
	)

	DescribeTable("Validate mandatory version semantic version format",
		func(
			givenVersion string,
			shouldSucceed bool,
		) {
			meta := builder.NewModuleReleaseMetaBuilder().
				WithName("test-version").
				WithNamespace(ControlPlaneNamespace).
				WithModuleName("test-module").
				WithMandatory(givenVersion).
				Build()

			err := kcpClient.Create(ctx, meta)
			if shouldSucceed {
				Expect(err).NotTo(HaveOccurred())
				err = kcpClient.Delete(ctx, meta)
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("spec.mandatory.version: Invalid value: \"%s\"",
					givenVersion)))
			}
		},
		Entry("when version is valid semantic version,"+
			thenNoErrors,
			"1.0.0",
			true,
		),
		Entry("when version has pre-release,"+
			thenNoErrors,
			"1.0.0-alpha",
			true,
		),
		Entry("when version has build metadata,"+
			thenNoErrors,
			"1.0.0+build.1",
			true,
		),
		Entry("when version has both pre-release and build metadata,"+
			thenNoErrors,
			"1.0.0-alpha.1+build.1",
			true,
		),
		Entry("when version has complex pre-release,"+
			thenNoErrors,
			"1.0.0-alpha.beta.1",
			true,
		),
		Entry("when version is empty,"+
			thenExpectAValidationError,
			"",
			false,
		),
		Entry("when version has only major,"+
			thenExpectAValidationError,
			"1",
			false,
		),
		Entry("when version has only major.minor,"+
			thenExpectAValidationError,
			"1.0",
			false,
		),
		Entry("when version has leading zeros in major,"+
			thenExpectAValidationError,
			"01.0.0",
			false,
		),
		Entry("when version has leading zeros in minor,"+
			thenExpectAValidationError,
			"1.01.0",
			false,
		),
		Entry("when version has leading zeros in patch,"+
			thenExpectAValidationError,
			"1.0.01",
			false,
		),
		Entry("when version has invalid characters,"+
			thenExpectAValidationError,
			"1.0.0@invalid",
			false,
		),
		Entry("when version starts with v,"+
			thenExpectAValidationError,
			"v1.0.0",
			false,
		),
		Entry("when version has trailing dot,"+
			thenExpectAValidationError,
			"1.0.0.",
			false,
		),
		Entry("when version has double dots,"+
			thenExpectAValidationError,
			"1..0.0",
			false,
		),
	)
})
