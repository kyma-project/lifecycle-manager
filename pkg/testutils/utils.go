package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

const (
	defaultBufferSize      = 2048
	Timeout                = time.Second * 10
	ConsistentCheckTimeout = time.Second * 10
	Interval               = time.Millisecond * 250
)

var (
	ErrNotFound                   = errors.New("resource does not exist")
	ErrNotDeleted                 = errors.New("resource has not been deleted")
	ErrDeletionTimestampFound     = errors.New("deletion timestamp not nil")
	ErrSampleCrNotInExpectedState = errors.New("resource not in expected state")
	ErrFetchingStatus             = errors.New("could not fetch status from resource")
)

func NewTestModule(name, channel string) v1beta2.Module {
	return NewTestModuleWithFixName(fmt.Sprintf("%s-%s", name, random.Name()), channel, "")
}

func NewTestModuleWithChannelVersion(name, channel, version string) v1beta2.Module {
	return NewTestModuleWithFixName(fmt.Sprintf("%s-%s", name, random.Name()), channel, version)
}

func NewTemplateOperatorWithVersion(version string) v1beta2.Module {
	return NewTestModuleWithFixName(TestModuleName, "", version)
}

func NewTemplateOperator(channel string) v1beta2.Module {
	return NewTestModuleWithFixName(TestModuleName, channel, "")
}

func NewTestModuleWithFixName(name, channel, version string) v1beta2.Module {
	return v1beta2.Module{
		Name:    name,
		Channel: channel,
		Managed: true,
		Version: version,
	}
}

func NewTestIssuer(namespace string) *certmanagerv1.Issuer {
	return &certmanagerv1.Issuer{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "test-issuer",
			Namespace: namespace,
			Labels: k8slabels.Set{
				shared.PurposeLabel: shared.CertManager,
				shared.ManagedBy:    shared.OperatorName,
			},
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}
}

func NewTestNamespace(namespace string) *apicorev1.Namespace {
	return &apicorev1.Namespace{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: namespace,
		},
	}
}

func AppendExternalCRDs(path string, files ...string) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, file := range files {
		crdPath := filepath.Join(path, file)
		moduleFile, err := os.Open(crdPath)
		if err != nil {
			return nil, err
		}
		decoder := machineryaml.NewYAMLOrJSONDecoder(moduleFile, defaultBufferSize)
		for {
			crd := &apiextensionsv1.CustomResourceDefinition{}
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

func DeletionTimeStampExists(ctx context.Context, group, version, kind, name, namespace string,
	clnt client.Client,
) (bool, error) {
	sampleCR := &unstructured.Unstructured{}
	sampleCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	if err := clnt.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, sampleCR); err != nil {
		return false, err
	}

	_, deletionTimestampExists, err := unstructured.NestedString(sampleCR.Object,
		"metadata", "deletionTimestamp")
	if err != nil || !deletionTimestampExists {
		return deletionTimestampExists, err
	}

	return deletionTimestampExists, err
}

func ApplyYAML(ctx context.Context, clnt client.Client, yamlFilePath string) error {
	resources, err := parseResourcesFromYAML(yamlFilePath, clnt)
	if err != nil {
		return err
	}

	for _, object := range resources {
		err := clnt.Patch(ctx, object, client.Apply, client.ForceOwnership, client.FieldOwner(shared.OperatorName))
		if err != nil {
			return fmt.Errorf("error applying patch to resource %s/%s: %w",
				object.GetNamespace(), object.GetName(), err)
		}
	}

	return nil
}

func parseResourcesFromYAML(yamlFilePath string, clnt client.Client) ([]*unstructured.Unstructured, error) {
	fileContent, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading YAML file '%s': %w", yamlFilePath, err)
	}
	yamlDocs := bytes.Split(fileContent, []byte("---"))

	decoder := serializer.NewCodecFactory(clnt.Scheme()).UniversalDeserializer()
	resources := make([]*unstructured.Unstructured, 0, len(yamlDocs))

	for _, doc := range yamlDocs {
		if len(doc) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{}
		_, _, err := decoder.Decode(doc, nil, obj)
		if err != nil {
			return nil, fmt.Errorf("error decoding YAML document: %w", err)
		}

		resources = append(resources, obj)
	}
	return resources, nil
}

func PatchServiceToTypeLoadBalancer(ctx context.Context, clnt client.Client, serviceName, namespace string) error {
	service := &apicorev1.Service{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: namespace}, service); err != nil {
		return err
	}

	service.Spec.Type = apicorev1.ServiceTypeLoadBalancer
	if err := clnt.Update(ctx, service); err != nil {
		return err
	}

	return nil
}

// ToStringList is a mapping function takes a slice of any type and a conversion function,
// and returns a slice of strings.
func ToStringList[T any](list []T, toString func(T) string) []string {
	if len(list) == 0 {
		return []string{}
	}
	result := make([]string, len(list))
	for i, item := range list {
		result[i] = toString(item)
	}
	return result
}
