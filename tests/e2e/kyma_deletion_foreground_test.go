package e2e_test

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("KCP Kyma CR Deletion With Foreground Propagation After SKR Cluster Removal", Ordered,
	func() {
		RunDeletionTest(apimetav1.DeletePropagationForeground)
	})
