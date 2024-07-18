package moduletemplate

import (
	"fmt"

	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
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
				WithVersion(givenVersion).
				WithModuleName("").
				WithChannel(module.Channel).
				WithOCM(compdescv2.SchemaVersion).Build()

			err := kcpClient.Create(ctx, template)
			if shouldSucceed {
				Expect(err).To(BeNil())
				err = kcpClient.Delete(ctx, template)
				Expect(err).To(BeNil())
			} else {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("spec.version: Invalid value: \"%s\"", givenVersion)))
			}
		},
		Entry("when version is empty,"+
			" then expect no validation errors",
			"",
			true,
		),
		Entry("when version contains one number,"+
			" then expect a validation error",
			"1",
			false,
		),
		Entry("when version contains two numbers,"+
			" then expect a validation error",
			"1.2",
			false,
		),
		Entry("when version contains three numbers,"+
			" then expect no validation errors",
			"1.2.3",
			true,
		),
		Entry("when version is a word,"+
			" then expect a validation error",
			"foo",
			false,
		),
		Entry("when version contains word and then semver,"+
			" then expect a validation error",
			"foo-1.2.3",
			false,
		),
		Entry("when version contains semver and then word,"+
			" then expect no validation errors",
			"1.2.3-foo",
			true,
		),
		Entry("when version contains semver and few words,"+
			" then expect no validation errors",
			"2.4.2-e2e-test",
			true,
		),
	)
	DescribeTable("Validate moduleName",
		func(
			givenModuleName string,
			shouldSucceed bool,
		) {
			template := builder.NewModuleTemplateBuilder().
				WithName(module.Name).
				WithModuleName(givenModuleName).
				WithVersion("").
				WithChannel(module.Channel).
				WithOCM(compdescv2.SchemaVersion).Build()

			err := kcpClient.Create(ctx, template)
			if shouldSucceed {
				Expect(err).To(BeNil())
				err = kcpClient.Delete(ctx, template)
				Expect(err).To(BeNil())
			} else {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("spec.moduleName: Invalid value: \"%s\"", givenModuleName)))
			}
		},
		Entry("when moduleName is empty,"+
			" then expect no validation errors",
			"",
			true,
		),
		Entry("when moduleName is a single letter,"+
			" then expect a validation error",
			"a",
			false,
		),
		Entry("when moduleName is a two-letter word,"+
			" then expect a validation error",
			"ab",
			false,
		),
		Entry("when moduleName is a three-letter word,"+
			" then expect no validation errors",
			"abc",
			true,
		),
		Entry("when moduleName contains a two-letter word,"+
			" then expect a validation error",
			"abc-def-gh",
			false,
		),
		Entry("when moduleName contains two words,"+
			" then expect no validation errors",
			"template-operator",
			true,
		),
		Entry("when moduleName contains a number,"+
			" then expect a validation error",
			"template-operator23",
			true,
		),
	)

})
