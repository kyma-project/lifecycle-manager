package controllers_test

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kyma-project/module-manager/operator/pkg/types"

	"github.com/kyma-project/lifecycle-manager/samples/template-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	sampleName = "kyma-sample"
)

var _ = Describe("Sample CR scenarios", Ordered, func() {

	DescribeTable("should set SampleCR to State `Ready`",
		func(sampleCR *v1alpha1.Sample) {
			// create SampleCR
			Expect(k8sClient.Create(ctx, sampleCR)).To(Succeed())

			Eventually(getPod(), 50*time.Second, 250*time.Millisecond).Should(BeTrue())
			crObjectKey := client.ObjectKeyFromObject(sampleCR)

			// check if SampleCR is Ready
			Eventually(sampleCRState(crObjectKey)).
				WithTimeout(60 * time.Second).
				WithPolling(5 * time.Second).
				Should(Equal(types.StateReady))

			// clean up sample CR
			Expect(k8sClient.Delete(ctx, sampleCR)).To(Succeed())
		}, sampleCREntries)

})

func getPod() func(g Gomega) bool {
	return func(g Gomega) bool {
		clientSet, err := kubernetes.NewForConfig(reconciler.Config)
		g.Expect(err).ToNot(HaveOccurred())

		pod, err := clientSet.CoreV1().Pods("redis").Get(ctx, "busybox-pod", metav1.GetOptions{})
		if err != nil {
			return false
		}

		pod.Status.Conditions = append(pod.Status.Conditions, v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		})

		_, err = clientSet.CoreV1().Pods("redis").UpdateStatus(ctx, pod, metav1.UpdateOptions{})
		g.Expect(err).ToNot(HaveOccurred())
		return true
	}
}

//nolint:gochecknoglobals
var sampleCREntries = createTableEntries([]string{sampleName})

func createTableEntries(sampleCRNames []string) []TableEntry {
	var tableEntries []TableEntry
	for _, sampleCRName := range sampleCRNames {
		entry := Entry(fmt.Sprintf("%s-CR-scenario", sampleCRName),
			createSampleCR(sampleCRName),
		)
		tableEntries = append(tableEntries, entry)
	}
	return tableEntries
}

func createSampleCR(sampleName string) *v1alpha1.Sample {
	return &v1alpha1.Sample{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1alpha1.SampleKind),
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sampleName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.SampleSpec{ReleaseName: "redis-release"},
	}
}

func sampleCRState(sampleObjKey client.ObjectKey) func(g Gomega) types.State {
	return func(g Gomega) types.State {
		sampleCR := &v1alpha1.Sample{}
		err := k8sClient.Get(ctx, sampleObjKey, sampleCR)
		if err != nil {
			//TODO
		}
		fmt.Fprintf(GinkgoWriter, "\n\nSAMPLECR: %#v\n\n", sampleCR)
		g.Expect(err).NotTo(HaveOccurred())
		return sampleCR.Status.State
	}
}
