package manifest_test

import (
	"os"
	"path/filepath"

	"ocm.software/ocm/api/utils/mime"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/status"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupTestEnvironment(ociTempDir, installName, mediaType string) {
	It("setup OCI", func() {
		var err error
		if mediaType == mime.MIME_TAR {
			err = PushToRemoteOCIRegistry(server, manifestTarPath, installName)
		} else {
			err = PushToRemoteOCIRegistry(server, manifestFilePath, installName)
		}
		Expect(err).NotTo(HaveOccurred())
	})
	BeforeEach(
		func() {
			Expect(os.RemoveAll(filepath.Join(os.TempDir(), ociTempDir))).To(Succeed())
		},
	)
}

var _ = Describe(
	"Rendering manifest install layer from raw file", Ordered, func() {
		ociTempDir := "main-dir"
		installName := filepath.Join(ociTempDir, "installs")
		var validManifest *v1beta2.Manifest
		setupTestEnvironment(ociTempDir, installName, mime.MIME_OCTET)

		Context("Given a Manifest CR", func() {
			It("When Manifest CR contains a valid install OCI image specification",
				func() {
					manifest, kyma := NewTestManifestWithParentKyma("oci")

					Eventually(CreateCR, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma).
						Should(Succeed())
					Eventually(AddManifestToKymaStatus, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), manifest.Name).
						Should(Succeed())
					Eventually(WithValidInstallImageSpecFromFile(ctx, kcpClient, installName,
						manifestFilePath,
						serverAddress, false), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Ready State", func() {
						Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateReady),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation exists", func() {
						Eventually(ExpectOCISyncRefAnnotationExists(ctx, kcpClient, true),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
			It("When Manifest CR contains a valid install OCI image specification and enabled deploy resource",
				func() {
					manifest, kyma := NewTestManifestWithParentKyma("oci")

					Eventually(CreateCR, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma).
						Should(Succeed())
					Eventually(AddManifestToKymaStatus, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), manifest.Name).
						Should(Succeed())
					Eventually(WithValidInstallImageSpecFromFile(ctx, kcpClient, installName,
						manifestFilePath,
						serverAddress, true), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					Eventually(func() error {
						var err error
						validManifest, err = GetManifestWithName(ctx, kcpClient, manifest.GetName())
						return err
					}).Should(Succeed())

					By("Then Manifest CR is in Ready State", func() {
						Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateReady),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation exists", func() {
						Eventually(ExpectOCISyncRefAnnotationExists(ctx, kcpClient, true),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
			It("When Manifest CR contains an invalid install OCI image specification and enabled deploy resource",
				func() {
					manifest, kyma := NewTestManifestWithParentKyma("oci")

					Eventually(CreateCR, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma).
						Should(Succeed())
					Eventually(AddManifestToKymaStatus, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), manifest.Name).
						Should(Succeed())
					Eventually(WithInvalidInstallImageSpec(ctx, kcpClient, false, manifestFilePath),
						standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Error State", func() {
						Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateError),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation does not exist", func() {
						Eventually(ExpectOCISyncRefAnnotationExists(ctx, kcpClient, false),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					By("And Manifest LastOperation is updated with error message", func() {
						Eventually(ExpectManifestLastOperationMessageContains,
							standardTimeout,
							standardInterval).
							WithContext(ctx).
							WithArguments(kcpClient, manifest.GetName(),
								"failed to extract raw manifest from layer digest").
							Should(Succeed())
					})

					By("When OCI Image is corrected", func() {
						Eventually(UpdateManifestSpec, standardTimeout, standardInterval).
							WithContext(ctx).
							WithArguments(kcpClient, manifest.GetName(), validManifest.Spec).
							Should(Succeed())
					})

					By("The Manifest CR is in Ready State and lastOperation is updated correctly", func() {
						Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateReady),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					By("And Manifest LastOperation is updated correctly", func() {
						Eventually(ExpectManifestLastOperationMessageContains,
							standardTimeout,
							standardInterval).
							WithContext(ctx).
							WithArguments(kcpClient, manifest.GetName(),
								status.ResourcesAreReadyMsg).
							Should(Succeed())
					})

					Eventually(DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
		})
	},
)

var _ = Describe(
	"Rendering manifest install layer from tar", Ordered, func() {
		ociTempDir := "main-dir"
		installName := filepath.Join(ociTempDir, "installs")
		setupTestEnvironment(ociTempDir, installName, mime.MIME_TAR)

		Context("Given a Manifest CR", func() {
			It("When Manifest CR contains a valid install OCI image specification",
				func() {
					manifest, kyma := NewTestManifestWithParentKyma("oci")

					Eventually(CreateCR, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma).
						Should(Succeed())
					Eventually(AddManifestToKymaStatus, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), manifest.Name).
						Should(Succeed())
					Eventually(WithValidInstallImageSpecFromTar(ctx, kcpClient, installName,
						manifestTarPath,
						serverAddress, false, false), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Ready State", func() {
						Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateReady),
							standardTimeout, standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
		})
	},
)
