package controllers_test

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kyma-project/lifecycle-manager/samples/template-operator/api/v1alpha1"
	"github.com/kyma-project/module-manager/operator/pkg/types"

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
			// pod which needs to be checked
			namespace := "redis"
			podName := "busybox-pod"

			// create SampleCR
			Expect(k8sClient.Create(ctx, sampleCR)).To(Succeed())

			// check if deployed Pod is up and running
			Eventually(getPod(namespace, podName)).
				WithTimeout(30 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(BeTrue())
			crObjectKey := client.ObjectKeyFromObject(sampleCR)

			// check if SampleCR is Ready
			Eventually(sampleCRState(crObjectKey)).
				WithTimeout(30 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(Equal(CRState{State: types.StateReady, Err: nil}))

			// clean up SampleCR
			Expect(k8sClient.Delete(ctx, sampleCR)).To(Succeed())

			// check if pod got deleted
			Eventually(checkDeleted(namespace, podName)).
				WithTimeout(30 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(BeTrue())
		}, sampleCREntries)

})

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

func getPod(namespace, podName string) func(g Gomega) bool {
	return func(g Gomega) bool {
		clientSet, err := kubernetes.NewForConfig(reconciler.Config)
		g.Expect(err).ToNot(HaveOccurred())

		pod, err := clientSet.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false
		}

		// Because there are no controllers monitoring built-in resources, state of objects do not get updated.
		// Thus, 'Ready'-State of pod needs to be set manually.
		pod.Status.Conditions = append(pod.Status.Conditions, v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		})

		_, err = clientSet.CoreV1().Pods(namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
		g.Expect(err).ToNot(HaveOccurred())
		return true
	}
}

type CRState struct {
	State types.State
	Err   error
}

func sampleCRState(sampleObjKey client.ObjectKey) func(g Gomega) CRState {
	return func(g Gomega) CRState {
		sampleCR := &v1alpha1.Sample{}
		err := k8sClient.Get(ctx, sampleObjKey, sampleCR)
		if err != nil {
			return CRState{State: types.StateError, Err: err}
		}
		g.Expect(err).NotTo(HaveOccurred())
		return CRState{State: sampleCR.Status.State, Err: nil}
	}
}

func checkDeleted(namespace, podName string) func(g Gomega) bool {
	return func(g Gomega) bool {
		clientSet, err := kubernetes.NewForConfig(reconciler.Config)
		g.Expect(err).ToNot(HaveOccurred())

		pod, err := clientSet.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false
		}
		// Because there are no controllers monitoring built-in resources, objects do not get deleted.
		// To check if resource is deleted, check for OwnerReference instead
		var nilReference []metav1.OwnerReference
		Expect(pod.ObjectMeta.OwnerReferences).Should(Equal(nilReference))
		return true
	}
}
