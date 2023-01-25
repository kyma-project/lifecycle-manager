package watcher_test

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
)

var _ = Describe("Create Watcher Certificates", Ordered, func() {
	tests := []struct {
		name           string
		namespace      *corev1.Namespace
		kyma           *v1alpha1.Kyma
		wantNewCertErr bool
		wantCreateErr  bool
		issuer         *v1.Issuer
	}{
		{
			name:      "Should create a valid CertificateCR",
			namespace: testutils.NewTestNamespace("testcase-1"),
			kyma: &v1alpha1.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-kyma-1",
					Namespace:   "testcase-1",
					Annotations: map[string]string{watcher.DomainAnnotation: "example.domain.com"},
				},
				Spec: v1alpha1.KymaSpec{
					Sync: v1alpha1.Sync{
						Enabled:      true,
						Strategy:     v1alpha1.SyncStrategyLocalClient,
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
			kyma: &v1alpha1.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-kyma-2",
					Namespace:   "testcase-2",
					Annotations: map[string]string{watcher.DomainAnnotation: "example.domain.com"},
				},
				Spec: v1alpha1.KymaSpec{
					Sync: v1alpha1.Sync{
						Enabled:      true,
						Strategy:     v1alpha1.SyncStrategyLocalClient,
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
			kyma: &v1alpha1.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kyma-3",
					Namespace: "testcase-3",
				},
				Spec: v1alpha1.KymaSpec{
					Sync: v1alpha1.Sync{
						Enabled:      true,
						Strategy:     v1alpha1.SyncStrategyLocalClient,
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
			name:           "Should fail since Kyma is nil",
			namespace:      testutils.NewTestNamespace("testcase-4"),
			kyma:           nil,
			issuer:         nil,
			wantCreateErr:  false,
			wantNewCertErr: true,
		},
	}
	for _, tt := range tests {
		test := tt
		It(test.name, func() {
			Expect(controlPlaneClient.Create(ctx, test.namespace)).Should(BeNil())
			if test.issuer != nil {
				Expect(controlPlaneClient.Create(ctx, test.issuer)).Should(BeNil())
			}
			cert, err := watcher.NewCertificate(controlPlaneClient, test.kyma)
			if test.wantNewCertErr {
				Expect(err).ShouldNot(BeNil())
				return
			}
			Expect(err).Should(BeNil())

			err = cert.Create(ctx, &watcher.SkrChartManagerConfig{WatcherLocalTestingEnabled: true})
			if test.wantCreateErr {
				Expect(err).ShouldNot(BeNil())
				return
			}
			Expect(err).Should(BeNil())

			if test.issuer != nil {
				Expect(controlPlaneClient.Delete(ctx, test.issuer)).Should(BeNil())
			}
		})

	}
})

// todo: KymaCR und skr secret erstellen und dann checken ob der sync klappt :D
