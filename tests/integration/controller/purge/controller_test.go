package purge_test

import (
	"context"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	testFinalizer = "purge.reconciler/test"
)

var _ = Describe("When kyma is not deleted within configured timeout", Ordered, func() {
	kyma := NewTestKyma("no-module-kyma")
	skrKyma := NewSKRKyma() // Only used to create the Issuers, not actually deployed itself
	var skrClient client.Client

	BeforeAll(ensureSetup(kyma, &skrClient))

	It("The purge logic should start after the timeout", func() {
		var issuer1 *unstructured.Unstructured
		var issuer2 *unstructured.Unstructured
		const retries = 2

		By("Create the Kyma object", func() {
			Expect(kcpClient.Create(ctx, kyma)).Should(Succeed())
			if updateRequired := kyma.EnsureLabelsAndFinalizers(); updateRequired {
				var err error
				for range retries {
					err = kcpClient.Update(ctx, kyma)
					if err == nil {
						break
					}
					err = kcpClient.Get(ctx, client.ObjectKeyFromObject(kyma), kyma)
					time.Sleep(5 * time.Millisecond)
				}
				Expect(err).ToNot(HaveOccurred())
			}
		})

		By("Create some CR with finalizer(s)", func() {
			issuer1 = createIssuerFor(skrKyma, "1")
			Expect(issuer1).NotTo(BeNil())
			Expect(skrClient.Create(ctx, issuer1)).Should(Succeed())
			Expect(getIssuerFinalizers(ctx, client.ObjectKeyFromObject(issuer1), skrClient)).
				Should(ContainElement(testFinalizer))

			issuer2 = createIssuerFor(skrKyma, "2")
			Expect(issuer2).NotTo(BeNil())
			Expect(skrClient.Create(ctx, issuer2)).Should(Succeed())
			Expect(getIssuerFinalizers(ctx, client.ObjectKeyFromObject(issuer2), skrClient)).
				Should(ContainElement(testFinalizer))
		})

		By("Kyma deletion is triggered", func() {
			err := kcpClient.Delete(ctx, kyma)
			Expect(err).ToNot(HaveOccurred())

			Eventually(updateKymaStatus(ctx, kcpClient, reconciler.UpdateStatus,
				client.ObjectKeyFromObject(kyma), shared.StateDeleting), Timeout, Interval).
				Should(Succeed())
		})

		By("Target finalizers should be dropped", func() {
			Eventually(KymaIsInState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateDeleting).
				Should(Succeed())
			Eventually(getIssuerFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(issuer1), skrClient).
				Should(BeEmpty())
			Eventually(getIssuerFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(issuer2), skrClient).
				Should(BeEmpty())
		})
	})
})

var _ = Describe("When kyma is deleted before configured timeout", Ordered, func() {
	kyma := NewTestKyma("drop-intantly-kyma")
	skrKyma := NewSKRKyma() // Only used to create the Issuers, not actually deployed itself
	var skrClient client.Client

	BeforeAll(ensureSetup(kyma, &skrClient))

	It("Should start purging right after the kyma is deleted", func() {
		var issuer1 *unstructured.Unstructured
		var issuer2 *unstructured.Unstructured

		By("Creating the kyma object first", func() {
			Expect(kcpClient.Create(ctx, kyma)).Should(Succeed())
			if updateRequired := kyma.EnsureLabelsAndFinalizers(); updateRequired {
				var err error
				for range 2 {
					err = kcpClient.Update(ctx, kyma)
					if err == nil {
						break
					}
					err = kcpClient.Get(ctx, client.ObjectKeyFromObject(kyma), kyma)
					time.Sleep(5 * time.Millisecond)
				}
				Expect(err).ToNot(HaveOccurred())
			}
		})

		By("Create some CR with finalizer(s)", func() {
			issuer1 = createIssuerFor(skrKyma, "1")
			Expect(issuer1).NotTo(BeNil())
			Expect(skrClient.Create(ctx, issuer1)).Should(Succeed())
			Expect(getIssuerFinalizers(ctx, client.ObjectKeyFromObject(issuer1), skrClient)).
				Should(ContainElement(testFinalizer))

			issuer2 = createIssuerFor(skrKyma, "2")
			Expect(issuer2).NotTo(BeNil())
			Expect(skrClient.Create(ctx, issuer2)).Should(Succeed())
			Expect(getIssuerFinalizers(ctx, client.ObjectKeyFromObject(issuer2), skrClient)).
				Should(ContainElement(testFinalizer))
		})

		By("Triggering kyma deletion and is completely removed", func() {
			//	Kyma delete event
			err := kcpClient.Delete(ctx, kyma)
			Expect(err).ToNot(HaveOccurred())
		})

		By("Target finalizers should be dropped immediately", func() {
			Eventually(getIssuerFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(issuer1), skrClient).
				Should(BeEmpty())

			Eventually(getIssuerFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(issuer2), skrClient).
				Should(BeEmpty())
		})
	})
})

var _ = Describe("When some important CRDs should be skipped", Ordered, func() {
	kyma := NewTestKyma("skip-crds-kyma")
	skrKyma := NewSKRKyma()
	var skrClient client.Client
	const retries = 5

	BeforeAll(ensureSetup(kyma, &skrClient))

	It("Should skip the CRDs passed into the Purge Reconciler", func() {
		var issuer1 *unstructured.Unstructured
		var issuer2 *unstructured.Unstructured
		var destRule1 *unstructured.Unstructured
		var destRule2 *unstructured.Unstructured

		By("Creating the kyma object first and adding custom finalizers to be skipped", func() {
			Expect(kcpClient.Create(ctx, kyma)).Should(Succeed())
			if updateRequired := kyma.EnsureLabelsAndFinalizers(); updateRequired {
				var err error
				for range retries {
					err = kcpClient.Update(ctx, kyma)
					if err == nil {
						break
					}
					err = kcpClient.Get(ctx, client.ObjectKeyFromObject(kyma), kyma)
					time.Sleep(5 * time.Millisecond)
				}
				Expect(err).ToNot(HaveOccurred())
			}
		})

		By("Create some CR with finalizer(s)", func() {
			issuer1 = createIssuerFor(skrKyma, "1")
			Expect(issuer1).NotTo(BeNil())
			Expect(skrClient.Create(ctx, issuer1)).Should(Succeed())
			Expect(getIssuerFinalizers(ctx, client.ObjectKeyFromObject(issuer1), skrClient)).
				Should(ContainElement(testFinalizer))

			issuer2 = createIssuerFor(skrKyma, "2")
			Expect(issuer2).NotTo(BeNil())
			Expect(skrClient.Create(ctx, issuer2)).Should(Succeed())
			Expect(getIssuerFinalizers(ctx, client.ObjectKeyFromObject(issuer2), skrClient)).
				Should(ContainElement(testFinalizer))
		})

		By("Creating CRs which shouldn't be touched", func() {
			destRule1 = createDestinationRuleFor(skrKyma, "1")
			Expect(destRule1).NotTo(BeNil())
			Expect(skrClient.Create(ctx, destRule1)).Should(Succeed())
			Expect(getDestinationRuleFinalizers(ctx, client.ObjectKeyFromObject(destRule1), skrClient)).
				Should(ContainElement(testFinalizer))

			destRule2 = createDestinationRuleFor(skrKyma, "2")
			Expect(destRule2).NotTo(BeNil())
			Expect(skrClient.Create(ctx, destRule2)).Should(Succeed())
			Expect(getDestinationRuleFinalizers(ctx, client.ObjectKeyFromObject(destRule2), skrClient)).
				Should(ContainElement(testFinalizer))
		})

		By("Triggering kyma deletion and is completely removed", func() {
			//	Kyma delete event
			err := kcpClient.Delete(ctx, kyma)
			Expect(err).ToNot(HaveOccurred())
		})

		By("Target finalizers should be dropped immediately", func() {
			Eventually(getIssuerFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(issuer1), skrClient).
				Should(BeEmpty())
			Eventually(getIssuerFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(issuer2), skrClient).
				Should(BeEmpty())
		})

		By("To-Skip CRDs should remain untouched", func() {
			Eventually(getDestinationRuleFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(destRule1), skrClient).
				ShouldNot(BeEmpty())
			Eventually(getDestinationRuleFinalizers, Timeout, Interval).
				WithContext(ctx).
				WithArguments(client.ObjectKeyFromObject(destRule2), skrClient).
				ShouldNot(BeEmpty())
		})
	})
})

func createDestinationRuleObj() *unstructured.Unstructured {
	gvk := schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1alpha3",
		Kind:    "DestinationRule",
	}
	res := unstructured.Unstructured{}
	res.SetGroupVersionKind(gvk)
	return &res
}

func createIssuerObj() *unstructured.Unstructured {
	gvk := schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Issuer",
	}
	res := unstructured.Unstructured{}
	res.SetGroupVersionKind(gvk)
	return &res
}

func createDestinationRuleFor(kyma *v1beta2.Kyma, nameSuffix string) *unstructured.Unstructured {
	res := createDestinationRuleObj()
	name := kyma.Name
	if nameSuffix != "" {
		name += nameSuffix
	}
	res.SetName(name)
	res.SetNamespace(kyma.Namespace)
	res.SetFinalizers([]string{testFinalizer})

	if err := unstructured.SetNestedMap(res.Object, map[string]any{}, "spec"); err != nil {
		return nil
	}
	if err := unstructured.SetNestedMap(res.Object, map[string]any{}, "spec", "trafficPolicy"); err != nil {
		return nil
	}
	if err := unstructured.SetNestedMap(res.Object,
		map[string]any{}, "spec", "trafficPolicy", "loadBalancer"); err != nil {
		return nil
	}
	if err := unstructured.SetNestedField(res.Object,
		"LEAST_REQUEST", "spec", "trafficPolicy", "loadBalancer", "simple"); err != nil {
		return nil
	}
	return res
}

func createIssuerFor(kyma *v1beta2.Kyma, nameSuffix string) *unstructured.Unstructured {
	res := createIssuerObj()
	name := kyma.Name
	if nameSuffix != "" {
		name += nameSuffix
	}
	res.SetName(name)
	res.SetNamespace(kyma.Namespace)
	res.SetFinalizers([]string{testFinalizer})

	if err := unstructured.SetNestedMap(res.Object, map[string]any{}, "spec"); err != nil {
		return nil
	}
	if err := unstructured.SetNestedMap(res.Object, map[string]any{}, "spec", "ca"); err != nil {
		return nil
	}
	if err := unstructured.SetNestedField(res.Object, "foobar", "spec", "ca", "secretName"); err != nil {
		return nil
	}

	return res
}

func getIssuerFinalizers(ctx context.Context, key client.ObjectKey, cl client.Client) []string {
	res := createIssuerObj()
	Expect(cl.Get(ctx, key, res)).Should(Succeed())
	return res.GetFinalizers()
}

func getDestinationRuleFinalizers(ctx context.Context, key client.ObjectKey, cl client.Client) []string {
	res := createDestinationRuleObj()
	Expect(cl.Get(ctx, key, res)).Should(Succeed())
	return res.GetFinalizers()
}

func updateKymaStatus(ctx context.Context, client client.Client, updateStatus func(context.Context, *v1beta2.Kyma,
	shared.State, string,
) error, key client.ObjectKey, state shared.State,
) func() error {
	return func() error {
		kyma := v1beta2.Kyma{}

		err := client.Get(ctx, key, &kyma)
		if err != nil {
			return err
		}

		err = updateStatus(ctx, &kyma, state, "TODO: Debugging")
		if err != nil {
			return err
		}

		return nil
	}
}

func ensureSetup(kyma *v1beta2.Kyma, skrClient *client.Client) func() {
	return func() {
		var err error
		Eventually(func() error {
			err = testSkrContextFactory.Init(ctx, kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())

		Eventually(func() error {
			*skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())

		Eventually(CreateNamespace, Timeout, Interval).
			WithContext(ctx).
			WithArguments(*skrClient, shared.DefaultRemoteNamespace).
			Should(Succeed())

		// Patching the SKR Cluster to have the necessary CRDs
		Eventually(func() error {
			externalCRDs, err := AppendExternalCRDs(
				filepath.Join(integration.GetProjectRoot(), "config", "samples", "tests", "crds"),
				"cert-manager-v1.10.1.crds.yaml",
				"istio-v1.17.1.crds.yaml")

			Expect(err).ShouldNot(HaveOccurred())

			for _, crd := range externalCRDs {
				skrClient := *skrClient
				err = skrClient.Create(ctx, crd)
				Expect(err).ShouldNot(HaveOccurred())
			}
			return err
		}, Timeout, Interval).Should(Succeed())
	}
}
