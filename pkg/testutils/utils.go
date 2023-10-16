//nolint:wrapcheck
package testutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apiExtensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

const (
	defaultBufferSize      = 2048
	Timeout                = time.Second * 40
	ConsistentCheckTimeout = time.Second * 10
	Interval               = time.Millisecond * 250
)

var (
	ErrNotFound                   = errors.New("resource not exists")
	ErrNotDeleted                 = errors.New("resource not deleted")
	ErrDeletionTimestampFound     = errors.New("deletion timestamp not nil")
	ErrEmptyRestConfig            = errors.New("rest.Config is nil")
	ErrDeletionTimestamp          = errors.New("DeletionTimeStamp does not exist or is not a string")
	ErrSampleCrNotInExpectedState = errors.New("resource not in expected state")
	ErrFetchingStatus             = errors.New("could not fetch status from resource")
)

func NewTestModule(name, channel string) v1beta2.Module {
	return v1beta2.Module{
		Name:    fmt.Sprintf("%s-%s", name, builder.RandomName()),
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

func CRExists(obj v1.Object, clientError error) error {
	if util.IsNotFound(clientError) {
		return ErrNotFound
	}
	if clientError != nil {
		return clientError
	}
	if obj != nil && obj.GetDeletionTimestamp() != nil {
		return ErrDeletionTimestampFound
	}
	if obj == nil {
		return ErrNotFound
	}
	return nil
}

func NewSKRCluster(scheme *k8sruntime.Scheme) (client.Client, *envtest.Environment, error) {
	skrEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := skrEnv.Start()
	if err != nil {
		return nil, nil, err
	}
	if cfg == nil {
		return nil, nil, ErrEmptyRestConfig
	}

	var authUser *envtest.AuthenticatedUser
	authUser, err = skrEnv.AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, cfg)
	if err != nil {
		return nil, nil, err
	}

	remote.LocalClient = func() *rest.Config {
		return authUser.Config()
	}

	skrClient, err := client.New(authUser.Config(), client.Options{Scheme: scheme})

	return skrClient, skrEnv, err
}

func AppendExternalCRDs(path string, files ...string) ([]*apiExtensionsv1.CustomResourceDefinition, error) {
	var crds []*apiExtensionsv1.CustomResourceDefinition
	for _, file := range files {
		crdPath := filepath.Join(path, file)
		moduleFile, err := os.Open(crdPath)
		if err != nil {
			return nil, err
		}
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
	return crds, nil
}

func DescriptorExistsInCache(moduleTemplate *v1beta2.ModuleTemplate) bool {
	moduleTemplateFromCache := moduleTemplate.GetDescFromCache()

	return moduleTemplateFromCache != nil
}

func GetDeletionTimeStamp(ctx context.Context, group, version, kind, name, namespace string,
	clnt client.Client,
) (string, error) {
	sampleCR := &unstructured.Unstructured{}
	sampleCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	if err := clnt.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, sampleCR); err != nil {
		return "", err
	}

	deletionTimestampFromCR, deletionTimestampExists, err := unstructured.NestedString(sampleCR.Object,
		"metadata", "deletionTimestamp")
	if err != nil || !deletionTimestampExists {
		return "", ErrDeletionTimestamp
	}

	return deletionTimestampFromCR, err
}

func CRIsInState(ctx context.Context, group, version, kind, name, namespace string, statusPath []string,
	clnt client.Client, expectedState string,
) error {
	resourceCR := &unstructured.Unstructured{}
	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})

	if err := clnt.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		return err
	}

	stateFromCR, stateExists, err := unstructured.NestedString(resourceCR.Object, statusPath...)
	if err != nil || !stateExists {
		return ErrFetchingStatus
	}

	if stateFromCR != expectedState {
		return fmt.Errorf("%w: expect %s, but in %s",
			ErrSampleCrNotInExpectedState, expectedState, stateFromCR)
	}
	return nil
}
