package certmanager_test

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/certmanager"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Create Watcher Certificates", Ordered, func() {

	type args struct {
		kyma *v1alpha1.Kyma
	}
	tests := []struct {
		name           string
		args           args
		wantNewCertErr bool
		wantCreateErr  bool
	}{
		// TODO: Add test cases.
		{
			name: "Certificate Secret should be synced to Remote Cluster",
			args: args{
				kyma: &v1alpha1.Kyma{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kyma-1",
						Namespace: "default",
					},
					Spec: v1alpha1.KymaSpec{
						Sync: v1alpha1.Sync{
							Enabled:      true,
							Strategy:     v1alpha1.SyncStrategyLocalClient,
							Namespace:    metav1.NamespaceDefault,
							NoModuleCopy: true,
						},
					},
					Status: v1alpha1.KymaStatus{},
				},
			},
			wantCreateErr:  false,
			wantNewCertErr: false,
		},
	}
	for _, tt := range tests {
		test := tt
		It("", func() {
			cert, err := certmanager.NewCertificate(ctx, controlPlaneClient, runtimeClient, test.args.kyma)
			if test.wantNewCertErr {
				Expect(err).ShouldNot(BeNil())
				return
			}
			Expect(err).Should(BeNil())

			err = cert.Create()
			if test.wantCreateErr {
				Expect(err).ShouldNot(BeNil())
			}
			Expect(err).Should(BeNil())

			Eventually(GetRemoteCertSecret(
				fmt.Sprintf("%s%s", test.args.kyma.Name, certmanager.CertificateSuffix),
				"default"), 20*time.Second, Interval).
				Should(BeEquivalentTo(string(v1alpha1.StateProcessing)))
		})

	}
})

func GetRemoteCertSecret(secretName, namespace string) func() bool {
	return func() bool {
		certSecret := &corev1.Secret{}
		if namespace == "" {
			namespace = corev1.NamespaceDefault
		}
		err := runtimeClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      secretName,
		}, certSecret)
		if err != nil || certSecret == nil {
			return false
		}
		return true
	}
}

// todo: KymaCR und skr secret erstellen und dann checken ob der sync klappt :D
