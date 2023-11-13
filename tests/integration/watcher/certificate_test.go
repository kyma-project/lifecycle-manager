package watcher_test

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create Watcher Certificates", Ordered, func() {
	tests := []struct {
		name           string
		namespace      *apicorev1.Namespace
		kyma           *v1beta2.Kyma
		wantNewCertErr bool
		wantCreateErr  bool
		issuer         *certmanagerv1.Issuer
	}{
		{
			name:      "Should create a valid CertificateCR",
			namespace: testutils.NewTestNamespace("testcase-1"),
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:        "test-kyma-1",
					Namespace:   "testcase-1",
					Annotations: map[string]string{watcher.DomainAnnotation: "example.domain.com"},
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
				ObjectMeta: apimetav1.ObjectMeta{
					Name:        "test-kyma-2",
					Namespace:   "testcase-2",
					Annotations: map[string]string{watcher.DomainAnnotation: "example.domain.com"},
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
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "test-kyma-3",
					Namespace: "testcase-3",
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
			cert, err := watcher.NewCertificateManager(controlPlaneClient,
				test.kyma, test.namespace.Name, test.namespace.Name, []string{})
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
