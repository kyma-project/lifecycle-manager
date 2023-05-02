package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dummy Test", Ordered, func() {
	It("Should be true", func() {
		test := true
		Expect(test).Should(BeTrue())
	})
})
