package withcertificatesync_test

import (
	"bytes"
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/certmanager"
)

var _ = Describe("Create Watcher Certificates", Ordered, func() {

	type testCase struct {
		name            string
		namespace       *corev1.Namespace
		secret          *corev1.Secret
		oldRemoteSecret *corev1.Secret
		kyma            *v1alpha1.Kyma
		checkFunc       func(testCase2 testCase)
	}
	tests := []testCase{
		{
			name:      "Should sync new Certificate Secret to Remote Cluster",
			namespace: createNamespace("testcase-1"),
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kyma-1-watcher-certificate",
					Namespace: "testcase-1",
					Labels: labels.Set{
						v1alpha1.PurposeLabel: v1alpha1.CertManager,
						v1alpha1.ManagedBy:    v1alpha1.OperatorName,
					},
				},
				Data: map[string][]byte{"ca.cert": []byte("example-certificate-content")},
			},
			kyma: createKyma("test-kyma-1", "testcase-1"),
			checkFunc: func(test testCase) {
				Eventually(isSecretInSync(ctx, runtimeClient, test.secret, test.kyma.Spec.Sync.Namespace),
					time.Minute*10, Interval).Should(BeTrue())
			},
		},
		{
			name:      "Should not sync Secret, since 'Purpose'-label is missing",
			namespace: createNamespace("testcase-2"),
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kyma-2-watcher-certificate",
					Namespace: "testcase-2",
					Labels: labels.Set{
						v1alpha1.ManagedBy: v1alpha1.OperatorName,
					},
				},
				Data: map[string][]byte{"ca.cert": []byte("example-certificate-content")},
			},
			kyma: createKyma("test-kyma-2", "testcase-2"),
			checkFunc: func(test testCase) {
				//make sure reconciliation had time to pick secret up
				time.Sleep(5 * time.Second)
				Eventually(Expect(errors.IsNotFound(
					runtimeClient.Get(ctx, types.NamespacedName{
						Namespace: test.kyma.Spec.Sync.Namespace,
						Name:      test.secret.Name,
					}, &corev1.Secret{}))).
					Should(BeTrue()))
			},
		},
		{
			name:      "Should update Remote Secret",
			namespace: createNamespace("testcase-3"),
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kyma-3-watcher-certificate",
					Namespace: "testcase-3",
					Labels: labels.Set{
						v1alpha1.PurposeLabel: v1alpha1.CertManager,
						v1alpha1.ManagedBy:    v1alpha1.OperatorName,
					},
					Annotations: map[string]string{"example-annotation": "abc123"},
				},
				Data: map[string][]byte{"ca.cert": []byte("example-certificate-content")},
			},
			oldRemoteSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kyma-3-watcher-certificate",
					Namespace: "testcase-3",
					Labels: labels.Set{
						v1alpha1.PurposeLabel: v1alpha1.CertManager,
					},
				},
				Data: map[string][]byte{"ca.cert": []byte("old-certificate-content")},
			},
			kyma: createKyma("test-kyma-3", "testcase-3"),
			checkFunc: func(test testCase) {
				Eventually(isSecretInSync(ctx, runtimeClient, test.secret, test.kyma.Spec.Sync.Namespace),
					time.Minute*10, Interval).Should(BeTrue())
			},
		},
	}
	for _, tt := range tests {
		test := tt
		It(test.name, func() {
			Expect(controlPlaneClient.Create(ctx, test.namespace)).Should(BeNil())
			Expect(controlPlaneClient.Create(ctx, test.kyma)).Should(BeNil())
			if test.oldRemoteSecret != nil {
				Expect(runtimeClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name: test.kyma.Spec.Sync.Namespace,
				}})).Should(BeNil())
				Expect(runtimeClient.Create(ctx, test.oldRemoteSecret)).Should(BeNil())
			}
			Expect(controlPlaneClient.Create(ctx, test.secret)).Should(BeNil())

			test.checkFunc(test)
		})

	}
})

func createNamespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

func createKyma(name, namespace string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{certmanager.DomainAnnotation: "example.domain.com"},
		},
		Spec: v1alpha1.KymaSpec{
			Channel: v1alpha1.DefaultChannel,
			Sync: v1alpha1.Sync{
				Enabled:      true,
				Strategy:     v1alpha1.SyncStrategyLocalClient,
				Namespace:    namespace,
				NoModuleCopy: true,
			},
		},
	}
}

func isSecretInSync(ctx context.Context, runtimeClient client.Client, localSecret *corev1.Secret, namespace string) func() bool {
	return func() bool {
		secretFromRemote, err := getSecret(ctx, runtimeClient, localSecret.Name, namespace)
		if err != nil {
			return false
		}
		for key, expectedValue := range localSecret.Data {
			if res := bytes.Compare(secretFromRemote.Data[key], expectedValue); res != 0 {
				return false
			}
		}
		for key, value := range localSecret.Labels {
			if secretFromRemote.Labels[key] != value {
				return false
			}
		}
		for key, value := range localSecret.Annotations {
			if secretFromRemote.Annotations[key] != value {
				return false
			}
		}
		return true
	}
}

func getSecret(ctx context.Context, testClient client.Client, name, namespace string) (*corev1.Secret, error) {
	secretInCluster := &corev1.Secret{}
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	err := testClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, secretInCluster)
	if err != nil {
		return nil, err
	}
	return secretInCluster, nil
}
