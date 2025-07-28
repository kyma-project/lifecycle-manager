package watcher_test

import (
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/config"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/secret"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/resources"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
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

			certificateConfig := config.CertificateValues{
				Duration:    1 * time.Hour,
				RenewBefore: 5 * time.Minute,
				KeySize:     flags.DefaultSelfSignedCertKeySize,
			}

			certificateManagerConfig := certificate.Config{
				SkrServiceName:     skrwebhookresources.SkrResourceName,
				SkrNamespace:       test.namespace.Name,
				AdditionalDNSNames: []string{},
				GatewaySecretName:  shared.GatewaySecretName,
				RenewBuffer:        flags.DefaultSelfSignedCertificateRenewBuffer,
			}

			certificateManager := certificate.NewSKRCertService(
				certmanager.NewCertificateRepository(controlPlaneClient,
					"klm-watcher-selfsigned",
					test.namespace.Name,
					certificateConfig,
				),
				secret.NewCertificateSecretRepository(controlPlaneClient, test.namespace.Name),
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
