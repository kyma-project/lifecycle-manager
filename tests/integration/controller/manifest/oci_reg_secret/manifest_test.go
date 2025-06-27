package oci_reg_secret_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/keychainprovider"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe(
	"Given manifest with private registry", func() {
		mainOciTempDir := "private-oci"
		installName := filepath.Join(mainOciTempDir, "crs")
		It(
			"setup remote oci Registry",
			func() {
				err := PushToRemoteOCIRegistry(server, manifestFilePath, installName)
				Expect(err).NotTo(HaveOccurred())
			},
		)
		BeforeEach(
			func() {
				Expect(os.RemoveAll(filepath.Join(os.TempDir(), mainOciTempDir))).To(Succeed())
			},
		)

		It("Manifest should be in Error state with no auth secret found error message", func() {
			manifestWithInstall, kyma := NewTestManifestWithParentKyma("private-oci-registry")

			Eventually(CreateCR, standardTimeout, standardInterval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma).
				Should(Succeed())
			Eventually(AddManifestToKymaStatus, standardTimeout, standardInterval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), manifestWithInstall.Name).
				Should(Succeed())
			Eventually(WithValidInstallImageSpecFromFile(ctx, kcpClient, installName, manifestFilePath,
				server.Listener.Addr().String(), false),
				standardTimeout,
				standardInterval).
				WithArguments(manifestWithInstall).Should(Succeed())
			Eventually(func() string {
				status, err := GetManifestStatus(ctx, kcpClient, manifestWithInstall.GetName())
				if err != nil {
					return err.Error()
				}

				if status.State != shared.StateError {
					return "manifest not in error state"
				}
				if strings.Contains(status.LastOperation.Operation, keychainprovider.ErrNoAuthSecretFound.Error()) {
					return keychainprovider.ErrNoAuthSecretFound.Error()
				}
				return status.LastOperation.Operation
			}, standardTimeout, standardInterval).
				Should(Equal(keychainprovider.ErrNoAuthSecretFound.Error()))
		})
	},
)
