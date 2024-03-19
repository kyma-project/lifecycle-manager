package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("KCP Kyma CR Deletion With Background Propagation After SKR Cluster Removal", Ordered,
	func() {
		RunDeletionTest(apimetav1.DeletePropagationBackground)
	})
