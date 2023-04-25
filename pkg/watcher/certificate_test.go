package watcher_test

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

var _ = Describe("Create Watcher Certificates", Ordered, func() {
	tests := []struct {
		name           string
		namespace      *corev1.Namespace
		kyma           *v1beta2.Kyma
		wantNewCertErr bool
		wantCreateErr  bool
		issuer         *v1.Issuer
	}{
		{
			name:      "Should create a valid CertificateCR",
			namespace: testutils.NewTestNamespace("testcase-1"),
			kyma: &v1beta2.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-kyma-1",
					Namespace:   "testcase-1",
					Annotations: map[string]string{watcher.DomainAnnotation: "example.domain.com"},
				},
				Spec: v1beta2.KymaSpec{
					Sync: v1beta2.Sync{
						Enabled:      true,
						Strategy:     v1beta2.SyncStrategyLocalClient,
						Namespace:    metav1.NamespaceDefault,
						NoModuleCopy: true,
					},
				},
			},
			issuer:         testutils.NewTestIssuer("testcase-1"),
			wantCreateErr:  false,
			wantNewCertErr: false,
		},
		{
			name:      "Should fail since no Issuer can be found",
			namespace: testutils.NewTestNamespace("testcase-2"),
			kyma: &v1beta2.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-kyma-2",
					Namespace:   "testcase-2",
					Annotations: map[string]string{watcher.DomainAnnotation: "example.domain.com"},
				},
				Spec: v1beta2.KymaSpec{
					Sync: v1beta2.Sync{
						Enabled:      true,
						Strategy:     v1beta2.SyncStrategyLocalClient,
						Namespace:    metav1.NamespaceDefault,
						NoModuleCopy: true,
					},
				},
			},
			issuer:         nil,
			wantCreateErr:  true,
			wantNewCertErr: false,
		},
		{
			name:      "Should fail since KymaCR is missing domain annotation",
			namespace: testutils.NewTestNamespace("testcase-3"),
			kyma: &v1beta2.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kyma-3",
					Namespace: "testcase-3",
				},
				Spec: v1beta2.KymaSpec{
					Sync: v1beta2.Sync{
						Enabled:      true,
						Strategy:     v1beta2.SyncStrategyLocalClient,
						Namespace:    metav1.NamespaceDefault,
						NoModuleCopy: true,
					},
				},
			},
			issuer:         nil,
			wantCreateErr:  true,
			wantNewCertErr: false,
		},
	}
	for _, tt := range tests {
		test := tt
		It(test.name, func() {
			Expect(controlPlaneClient.Create(ctx, test.namespace)).Should(Succeed())
			if test.issuer != nil {
				Expect(controlPlaneClient.Create(ctx, test.issuer)).Should(Succeed())
			}
			cert, err := watcher.NewCertificateManager(controlPlaneClient, test.kyma, test.namespace.Name, true)
			if test.wantNewCertErr {
				Expect(err).Should(HaveOccurred())
				return
			}
			Expect(err).ShouldNot(HaveOccurred())

			err = cert.Create(ctx)
			if test.wantCreateErr {
				Expect(err).Should(HaveOccurred())
				return
			}
			Expect(err).ShouldNot(HaveOccurred())

			if test.issuer != nil {
				Expect(controlPlaneClient.Delete(ctx, test.issuer)).Should(Succeed())
			}
		})

	}
})
