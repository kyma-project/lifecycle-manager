package kcp_test

import (
	"context"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var errLabelNotExist = errors.New("label does not exist on namespace")

var _ = Describe("Remote Namespace is correctly labelled", Ordered, func() {
	kyma := NewTestKyma("kyma-namespace-labels")
	var skrClient client.Client
	var err error
	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())
	})

	It("kyma-system namespace should have the istio and warden labels", func() {
		expectedLabels := map[string]string{
			"istio-injection": "enabled",
			"namespaces.warden.kyma-project.io/validate": "enabled",
		}
		Eventually(namespaceHasExpectedLabels, Timeout, Interval).WithContext(ctx).WithArguments(skrClient,
			RemoteNamespace, expectedLabels).Should(Succeed())
	})
})

func namespaceHasExpectedLabels(ctx context.Context, clnt client.Client,
	kymaNamespace string, expectedLabels map[string]string,
) error {
	var namespace apicorev1.Namespace
	err := clnt.Get(ctx, client.ObjectKey{Name: kymaNamespace}, &namespace)
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", kymaNamespace, err)
	}

	if namespace.Labels == nil {
		return errLabelNotExist
	}

	for k, v := range expectedLabels {
		if namespace.Labels[k] != v {
			return fmt.Errorf("label %s has value %s, expected %s", k, namespace.Labels[k], v)
		}
	}

	return nil
}
