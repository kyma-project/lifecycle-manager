package lookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	kymalookupsvc "github.com/kyma-project/lifecycle-manager/internal/service/kyma/lookup"
)

const (
	testKymaNamePrefix = "kyma-instance-"
	testKymaNamespace  = "kyma-namespace"
)

func Test_ByRuntimeID_ReturnsKymaInstance_WhenSingleInstanceFound(t *testing.T) {
	service := kymalookupsvc.NewService(&lookupRepoStub{singleKymaInstance()})
	runtimeID := "test-runtime-id-123"
	kymaInstance, err := service.ByRuntimeID(context.Background(), runtimeID)

	require.NoError(t, err)
	require.NotNil(t, kymaInstance)
	assert.Equal(t, testKymaNamePrefix+"1", kymaInstance.Name)
	assert.Equal(t, testKymaNamespace, kymaInstance.Namespace)
}

func Test_ByRuntimeID_ReturnsNotFoundError_WhenNoInstanceFound(t *testing.T) {
	service := kymalookupsvc.NewService(&lookupRepoStub{})
	runtimeID := "test-runtime-id-123"
	_, err := service.ByRuntimeID(context.Background(), runtimeID)

	require.Error(t, err)
	require.ErrorIs(t, err, kymalookupsvc.ErrNotFound)
}

func Test_ByRuntimeID_ReturnsMultipleFoundError_WhenMultipleInstancesFound(t *testing.T) {
	service := kymalookupsvc.NewService(&lookupRepoStub{twoKymaInstances()})
	runtimeID := "test-runtime-id-123"
	_, err := service.ByRuntimeID(context.Background(), runtimeID)

	require.Error(t, err)
	require.ErrorIs(t, err, kymalookupsvc.ErrMultipleFound)
}

func Test_ByRuntimeID_ReturnsLookupError_WhenRepositoryReturnsError(t *testing.T) {
	service := kymalookupsvc.NewService(&lookupRepoErrorStub{})
	runtimeID := "test-runtime-id-123"
	_, err := service.ByRuntimeID(context.Background(), runtimeID)

	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
}

type lookupRepoStub struct {
	expected []v1beta2.Kyma
}

func (lrs *lookupRepoStub) LookupByLabel(
	ctx context.Context, labelKey, labelValue string,
) (*v1beta2.KymaList, error) {
	return &v1beta2.KymaList{Items: lrs.expected}, nil
}

func singleKymaInstance() []v1beta2.Kyma {
	return []v1beta2.Kyma{
		{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      testKymaNamePrefix + "1",
				Namespace: testKymaNamespace,
			},
		},
	}
}

type lookupRepoErrorStub struct{}

func (lres *lookupRepoErrorStub) LookupByLabel(
	ctx context.Context, labelKey, labelValue string,
) (*v1beta2.KymaList, error) {
	return nil, assert.AnError
}

func twoKymaInstances() []v1beta2.Kyma {
	return []v1beta2.Kyma{
		{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      testKymaNamePrefix + "1",
				Namespace: testKymaNamespace,
			},
		},
		{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      testKymaNamePrefix + "2",
				Namespace: testKymaNamespace,
			},
		},
	}
}
