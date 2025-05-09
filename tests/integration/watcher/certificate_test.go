package watcher_test

import (
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	certrepo "github.com/kyma-project/lifecycle-manager/internal/repository/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	certsvc "github.com/kyma-project/lifecycle-manager/internal/service/certificate"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
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
			namespace: NewTestNamespace("testcase-1"),
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:        "test-kyma-1",
					Namespace:   "testcase-1",
					Annotations: map[string]string{shared.SkrDomainAnnotation: "example.domain.com"},
				},
			},
			issuer:         NewTestIssuer("testcase-1"),
			wantCreateErr:  false,
			wantNewCertErr: false,
		},
		{
			name:      "Should fail since KymaCR is missing domain annotation",
			namespace: NewTestNamespace("testcase-2"),
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "test-kyma-2",
					Namespace: "testcase-2",
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

			certificateConfig := certrepo.CertificateConfig{
				Duration:    1 * time.Hour,
				RenewBefore: 5 * time.Minute,
				KeySize:     flags.DefaultSelfSignedCertKeySize,
			}

			certificateManagerConfig := certsvc.CertificateManagerConfig{
				SkrServiceName:               watcher.SkrResourceName,
				SkrNamespace:                 test.namespace.Name,
				CertificateNamespace:         test.namespace.Name,
				AdditionalDNSNames:           []string{},
				GatewaySecretName:            shared.GatewaySecretName,
				RenewBuffer:                  flags.DefaultSelfSignedCertificateRenewBuffer,
				SkrCertificateNamingTemplate: "%s-webhook-tls",
			}

			certificateManager := certsvc.NewCertificateManager(
				certmanager.NewCertificate(controlPlaneClient,
					"klm-watcher-selfsigned",
					certificateConfig,
				),
				secret.NewCertificateSecretClient(controlPlaneClient),
				certificateManagerConfig,
			)

			err := certificateManager.CreateSkrCertificate(ctx, test.kyma)
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
