//nolint:wrapcheck
package testutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	. "github.com/onsi/gomega" //nolint:stylecheck,revive
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	compdesc2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	corev1 "k8s.io/api/core/v1"
	apiExtensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

const (
	randomStringLength     = 8
	letterBytes            = "abcdefghijklmnopqrstuvwxyz"
	defaultBufferSize      = 2048
	Timeout                = time.Second * 40
	ConsistentCheckTimeout = time.Second * 10
	Interval               = time.Millisecond * 250
)

var (
	ErrNotFound           = errors.New("resource not exists")
	ErrNotDeleted         = errors.New("resource not deleted")
	ErrManifestNotinState = errors.New("manifest is not in correct state")
)

func NewTestKyma(name string) *v1beta2.Kyma {
	return newKCPKymaWithNamespace(name, v1.NamespaceDefault, v1beta2.DefaultChannel, v1beta2.SyncStrategyLocalClient)
}

func NewKymaForE2E(name, namespace, channel string) *v1beta2.Kyma {
	kyma := newKCPKymaWithNamespace(name, namespace, channel, v1beta2.SyncStrategyLocalSecret)
	kyma.Labels[v1beta2.SyncLabel] = v1beta2.EnableLabelValue
	return kyma
}

func newKCPKymaWithNamespace(name, namespace, channel, syncStrategy string) *v1beta2.Kyma {
	return &v1beta2.Kyma{
		TypeMeta: v1.TypeMeta{
			APIVersion: v1beta2.GroupVersion.String(),
			Kind:       string(v1beta2.KymaKind),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, randString(randomStringLength)),
			Namespace: namespace,
			Annotations: map[string]string{
				watcher.DomainAnnotation:       "example.domain.com",
				v1beta2.SyncStrategyAnnotation: syncStrategy,
			},
			Labels: map[string]string{
				v1beta2.InstanceIDLabel: "test-instance",
			},
		},
		Spec: v1beta2.KymaSpec{
			Modules: []v1beta2.Module{},
			Channel: channel,
		},
	}
}

func NewTestManifest(prefix string) *v1beta2.Manifest {
	return &v1beta2.Manifest{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", prefix, randString(randomStringLength)),
			Namespace: v1.NamespaceDefault,
			Labels: map[string]string{
				v1beta2.KymaName: string(uuid.NewUUID()),
			},
			Annotations: map[string]string{},
		},
	}
}

func NewTestModuleCR(name, namespace, version, kind string) unstructured.Unstructured {
	moduleCR := unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1beta2.GroupVersion.Group,
		Version: version,
		Kind:    kind,
	})
	moduleCR.SetName(name)
	moduleCR.SetNamespace(namespace)
	return moduleCR
}

func NewTestModule(name, channel string) v1beta2.Module {
	return v1beta2.Module{
		Name:    fmt.Sprintf("%s-%s", name, randString(randomStringLength)),
		Channel: channel,
	}
}

func NewTestIssuer(namespace string) *certmanagerv1.Issuer {
	return &certmanagerv1.Issuer{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-issuer",
			Namespace: namespace,
			Labels:    watcher.LabelSet,
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}
}

func NewTestNamespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: namespace,
		},
	}
}

func NewUniqModuleName() string {
	return randString(randomStringLength)
}

// randomName creates a random string [a-z] with a length of 8.
func randomName() string {
	return randString(randomStringLength)
}

//nolint:unparam
func randString(length int) string {
	b := make([]byte, length)
	for i := range b {
		//nolint:gosec
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func DeployModuleTemplates(
	ctx context.Context,
	kcpClient client.Client,
	kyma *v1beta2.Kyma,
	onPrivateRepo,
	isInternal,
	isBeta bool,
	isClusterScoped bool,
) {
	for _, module := range kyma.Spec.Modules {
		Eventually(DeployModuleTemplate, Timeout, Interval).WithContext(ctx).
			WithArguments(kcpClient, module, onPrivateRepo, isInternal, isBeta, isClusterScoped).
			Should(Succeed())
	}
}

func DeployModuleTemplate(
	ctx context.Context,
	kcpClient client.Client,
	module v1beta2.Module,
	onPrivateRepo,
	isInternal,
	isBeta bool,
	isClusterScoped bool,
) error {
	template, err := ModuleTemplateFactory(module, unstructured.Unstructured{}, onPrivateRepo, isInternal, isBeta,
		isClusterScoped)
	if err != nil {
		return err
	}

	return kcpClient.Create(ctx, template)
}

func DeleteModuleTemplates(
	ctx context.Context,
	kcpClient client.Client,
	kyma *v1beta2.Kyma,
	onPrivateRepo bool,
) {
	for _, module := range kyma.Spec.Modules {
		template, err := ModuleTemplateFactory(module, unstructured.Unstructured{}, onPrivateRepo, false, false, false)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, template).Should(Succeed())
	}
}

func DeleteCR(ctx context.Context, clnt client.Client, obj client.Object) error {
	if err := clnt.Delete(ctx, obj); util.IsNotFound(err) {
		return nil
	}
	if err := clnt.Get(ctx, client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj); err != nil {
		if util.IsNotFound(err) {
			return nil
		}
		return err
	}
	return fmt.Errorf("%s/%s: %w", obj.GetNamespace(), obj.GetName(), ErrNotDeleted)
}

func CreateCR(ctx context.Context, clnt client.Client, obj client.Object) error {
	err := clnt.Create(ctx, obj)
	if !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func SyncKyma(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Name:      kyma.Name,
		Namespace: kyma.Namespace,
	}, kyma)
	// It might happen in some test case, kyma get deleted, if you need to make sure Kyma should exist,
	// write expected condition to check it specifically.
	return client.IgnoreNotFound(err)
}

func GetKyma(ctx context.Context, testClient client.Client, name, namespace string) (*v1beta2.Kyma, error) {
	kymaInCluster := &v1beta2.Kyma{}
	if namespace == "" {
		namespace = v1.NamespaceDefault
	}
	err := testClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, kymaInCluster)
	if err != nil {
		return nil, err
	}
	return kymaInCluster, nil
}

func IsKymaInState(ctx context.Context, kcpClient client.Client, kymaName string, state v1beta2.State) bool {
	kymaFromCluster, err := GetKyma(ctx, kcpClient, kymaName, "")
	if err != nil || kymaFromCluster.Status.State != state {
		return false
	}
	return true
}

func GetManifestSpecRemote(
	ctx context.Context,
	clnt client.Client,
	kyma *v1beta2.Kyma,
	module v1beta2.Module,
) (bool, error) {
	manifest, err := GetManifest(ctx, clnt, kyma, module)
	if err != nil {
		return false, err
	}
	return manifest.Spec.Remote, nil
}

func GetManifest(ctx context.Context,
	clnt client.Client,
	kyma *v1beta2.Kyma,
	module v1beta2.Module,
) (*v1beta2.Manifest, error) {
	template, err := ModuleTemplateFactory(module, unstructured.Unstructured{}, false, false, false, false)
	if err != nil {
		return nil, err
	}
	descriptor, err := template.GetDescriptor()
	if err != nil {
		return nil, err
	}
	manifest := &v1beta2.Manifest{}
	err = clnt.Get(
		ctx, client.ObjectKey{
			Namespace: kyma.Namespace,
			Name:      common.CreateModuleName(descriptor.GetName(), kyma.Name, module.Name),
		}, manifest,
	)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func ModuleTemplateFactory(
	module v1beta2.Module,
	data unstructured.Unstructured,
	onPrivateRepo bool,
	isInternal bool,
	isBeta bool,
	isClusterScoped bool,
) (*v1beta2.ModuleTemplate, error) {
	template, err := ModuleTemplateFactoryForSchema(module, data, compdesc2.SchemaVersion, onPrivateRepo)
	if err != nil {
		return nil, err
	}
	if isInternal {
		template.Labels[v1beta2.InternalLabel] = v1beta2.EnableLabelValue
	}
	if isBeta {
		template.Labels[v1beta2.BetaLabel] = v1beta2.EnableLabelValue
	}
	if isClusterScoped {
		if template.Annotations == nil {
			template.Annotations = make(map[string]string)
		}
		template.Annotations[v1beta2.IsClusterScopedAnnotation] = v1beta2.EnableLabelValue
	}
	return template, nil
}

func ModuleTemplateFactoryForSchema(
	module v1beta2.Module,
	data unstructured.Unstructured,
	schemaVersion compdesc.SchemaVersion,
	onPrivateRepo bool,
) (*v1beta2.ModuleTemplate, error) {
	var moduleTemplate v1beta2.ModuleTemplate
	var err error
	switch schemaVersion {
	case compdesc2.SchemaVersion:
		err = readModuleTemplateWithV2Schema(&moduleTemplate)
	case v3alpha1.GroupVersion:
		fallthrough
	case v3alpha1.SchemaVersion:
		fallthrough
	default:
		err = readModuleTemplateWithV3Schema(&moduleTemplate)
	}
	if onPrivateRepo {
		err = readModuleTemplateWithinPrivateRepo(&moduleTemplate)
	}
	if err != nil {
		return &moduleTemplate, err
	}
	moduleTemplate.Name = module.Name
	if moduleTemplate.Labels == nil {
		moduleTemplate.Labels = make(map[string]string)
	}
	moduleTemplate.Labels[v1beta2.ModuleName] = module.Name
	moduleTemplate.Spec.Channel = module.Channel
	if data.GetKind() != "" {
		moduleTemplate.Spec.Data = data
	}
	return &moduleTemplate, nil
}

func readModuleTemplateWithV2Schema(moduleTemplate *v1beta2.ModuleTemplate) error {
	template := "operator_v1beta2_moduletemplate_kcp-module.yaml"
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}

	modulePath := filepath.Join(filepath.Dir(filename),
		"../../config/samples/component-integration-installed", template)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(moduleFile, &moduleTemplate)
	return err
}

func readModuleTemplateWithinPrivateRepo(moduleTemplate *v1beta2.ModuleTemplate) error {
	template := "operator_v1beta2_moduletemplate_kcp-module-cred-label.yaml"
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(
		filepath.Dir(filename), "../../config/samples/component-integration-installed", template,
	)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(moduleFile, &moduleTemplate)
	return err
}

func readModuleTemplateWithV3Schema(moduleTemplate *v1beta2.ModuleTemplate) error {
	template := "operator_v1beta2_moduletemplate_ocm.software.v3alpha1.yaml"
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(
		filepath.Dir(filename), "../../config/samples/component-integration-installed", template,
	)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(moduleFile, &moduleTemplate)
	return err
}

func NewSKRCluster(scheme *k8sruntime.Scheme) (client.Client, *envtest.Environment) {
	skrEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := skrEnv.Start()
	Expect(cfg).NotTo(BeNil())
	Expect(err).NotTo(HaveOccurred())

	var authUser *envtest.AuthenticatedUser
	authUser, err = skrEnv.AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, cfg)
	Expect(err).NotTo(HaveOccurred())

	remote.LocalClient = func() *rest.Config {
		return authUser.Config()
	}

	skrClient, err := client.New(authUser.Config(), client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	return skrClient, skrEnv
}

func AppendExternalCRDs(path string, files ...string) []*apiExtensionsv1.CustomResourceDefinition {
	var crds []*apiExtensionsv1.CustomResourceDefinition
	for _, file := range files {
		crdPath := filepath.Join(path, file)
		moduleFile, err := os.Open(crdPath)
		Expect(err).ToNot(HaveOccurred())
		decoder := yaml.NewYAMLOrJSONDecoder(moduleFile, defaultBufferSize)
		for {
			crd := &apiExtensionsv1.CustomResourceDefinition{}
			if err = decoder.Decode(crd); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}
			crds = append(crds, crd)
		}
	}
	return crds
}

func ExpectKymaManagerField(
	ctx context.Context, controlPlaneClient client.Client, kymaName string, managerName string,
) (bool, error) {
	createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return false, err
	}
	if createdKyma.ManagedFields == nil {
		return false, nil
	}

	for _, v := range createdKyma.ManagedFields {
		if v.Subresource == "status" && v.Manager == managerName {
			return true, nil
		}
	}

	return false, nil
}

func GetModuleTemplate(ctx context.Context,
	clnt client.Client, name, namespace string,
) (*v1beta2.ModuleTemplate, error) {
	moduleTemplateInCluster := &v1beta2.ModuleTemplate{}
	moduleTemplateInCluster.SetNamespace(namespace)
	moduleTemplateInCluster.SetName(name)
	err := clnt.Get(ctx, client.ObjectKeyFromObject(moduleTemplateInCluster), moduleTemplateInCluster)
	if err != nil {
		return nil, err
	}
	return moduleTemplateInCluster, nil
}

func ManifestExists(ctx context.Context,
	kyma *v1beta2.Kyma, module v1beta2.Module, controlPlaneClient client.Client,
) error {
	_, err := GetManifest(ctx, controlPlaneClient, kyma, module)
	if util.IsNotFound(err) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return nil
}

func ModuleTemplateExists(ctx context.Context, client client.Client, name, namespace string) error {
	_, err := GetModuleTemplate(ctx, client, name, namespace)
	if util.IsNotFound(err) {
		return ErrNotFound
	}
	return nil
}

func AllModuleTemplatesExists(ctx context.Context,
	clnt client.Client, kyma *v1beta2.Kyma, remoteSyncNamespace string,
) error {
	for _, module := range kyma.Spec.Modules {
		if err := ModuleTemplateExists(ctx, clnt, module.Name, remoteSyncNamespace); err != nil {
			return err
		}
	}

	return nil
}

func UpdateManifestState(
	ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma, module v1beta2.Module, state v1beta2.State,
) error {
	kyma, err := GetKyma(ctx, clnt, kyma.GetName(), kyma.GetNamespace())
	if err != nil {
		return err
	}
	component, err := GetManifest(ctx, clnt, kyma, module)
	if err != nil {
		return err
	}
	component.Status.State = declarative.State(state)
	return clnt.Status().Update(ctx, component)
}
