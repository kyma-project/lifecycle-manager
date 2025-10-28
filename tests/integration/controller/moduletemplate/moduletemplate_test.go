package moduletemplate_test

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

var _ = Describe("ModuleTemplate version is not empty", Ordered, func() {
	module := NewTestModule("invalid-module", v1beta2.DefaultChannel)

	DescribeTable("Validate version",
		func(
			givenVersion string,
			shouldSucceed bool,
		) {
			template := builder.NewModuleTemplateBuilder().
				WithName(module.Name).
				WithNamespace(ControlPlaneNamespace).
				WithVersion(givenVersion).
				WithModuleName("").
				WithChannel(module.Channel).
				Build()

			err := kcpClient.Create(ctx, template)
			if shouldSucceed {
				Expect(err).NotTo(HaveOccurred())
				err = kcpClient.Delete(ctx, template)
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("spec.version: Invalid value: \"%s\"",
					givenVersion)))
			}
		},
		Entry("when version is empty,"+
			thenNoErrors,
			"",
			true,
		),
		Entry("when version contains one number,"+
			thenExpectAValidationError,
			"1",
			false,
		),
		Entry("when version contains two numbers,"+
			thenExpectAValidationError,
			"1.2",
			false,
		),
		Entry("when version contains three numbers,"+
			thenNoErrors,
			"1.2.3",
			true,
		),
		Entry("when version is a word,"+
			thenExpectAValidationError,
			"foo",
			false,
		),
		Entry("when version contains word and then semver,"+
			thenExpectAValidationError,
			"foo-1.2.3",
			false,
		),
		Entry("when version contains semver and then a word,"+
			thenNoErrors,
			"1.2.3-foo",
			true,
		),
		Entry("when version contains semver and two words,"+
			thenNoErrors,
			"2.4.2-e2e-test",
			true,
		),
		Entry("when version contains semver and three words,"+
			thenNoErrors,
			"2.4.2-e2e-test-foo",
			true,
		),
		Entry("when version contains lots of dashes"+
			thenNoErrors,
			"1.2.3-----foo--bar-----baz",
			true,
		),
		Entry("when version contains semver with leading zero in the major"+
			thenExpectAValidationError,
			"01.1.1",
			false,
		),
		Entry("when version contains semver with leading zero in the minor"+
			thenExpectAValidationError,
			"0.01.1",
			false,
		),
		Entry("when version contains semver with leading zero in the patch"+
			thenExpectAValidationError,
			"0.0.01",
			false,
		),
		Entry("when version contains semver with trailing letters in the patch"+
			thenExpectAValidationError,
			"0.0.1asdf",
			false,
		),
	)
	DescribeTable("Validate moduleName",
		func(
			givenModuleName string,
			shouldSucceed bool,
		) {
			template := builder.NewModuleTemplateBuilder().
				WithName(module.Name).
				WithNamespace(ControlPlaneNamespace).
				WithModuleName(givenModuleName).
				WithVersion("").
				WithChannel(module.Channel).
				Build()

			err := kcpClient.Create(ctx, template)
			if shouldSucceed {
				Expect(err).NotTo(HaveOccurred())
				err = kcpClient.Delete(ctx, template)
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("spec.moduleName: Invalid value: \"%s\"",
					givenModuleName)))
			}
		},
		Entry("when moduleName is empty,"+
			thenNoErrors,
			"",
			true,
		),
		Entry("when moduleName is a single letter,"+
			thenExpectAValidationError,
			"a",
			false,
		),
		Entry("when moduleName is a two-letter word,"+
			thenExpectAValidationError,
			"ab",
			false,
		),
		Entry("when moduleName is a three-letter word,"+
			thenNoErrors,
			"abc",
			true,
		),
		Entry("when moduleName contains a two-letter word,"+
			thenExpectAValidationError,
			"abc-def-gh",
			false,
		),
		Entry("when moduleName contains two words,"+
			thenNoErrors,
			TestModuleName,
			true,
		),
		Entry("when moduleName contains a number,"+
			thenExpectAValidationError,
			"template-operator23",
			false,
		),
	)
})
