package modulereleasemeta_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	mrmwatch "github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta"
)

type stubMrmRepo struct {
	mrm *v1beta2.ModuleReleaseMeta
	err error
}

func (s *stubMrmRepo) Get(_ context.Context, _ string) (*v1beta2.ModuleReleaseMeta, error) {
	return s.mrm, s.err
}

var errMrmRepo = errors.New("mrm repo error")

func TestMandatoryMrmChangeHandler_Watch(t *testing.T) {
	mandatory := &v1beta2.Mandatory{}

	tests := []struct {
		name      string
		mrmRepo   *stubMrmRepo
		kymaRepo  *stubKymaRepo
		wantCount int
	}{
		{
			name: "mandatory mrm with kymas - all kymas returned",
			mrmRepo: &stubMrmRepo{
				mrm: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{Mandatory: mandatory},
				},
			},
			kymaRepo:  &stubKymaRepo{kymaList: kymaListWith("kyma-1", "kyma-2")},
			wantCount: 2,
		},
		{
			name: "mrm mandatory is nil - no requests returned",
			mrmRepo: &stubMrmRepo{
				mrm: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{Mandatory: nil},
				},
			},
			kymaRepo:  &stubKymaRepo{kymaList: kymaListWith("kyma-1")},
			wantCount: 0,
		},
		{
			name:      "mrmRepo error - nil returned",
			mrmRepo:   &stubMrmRepo{err: errMrmRepo},
			kymaRepo:  &stubKymaRepo{kymaList: kymaListWith("kyma-1")},
			wantCount: 0,
		},
		{
			name: "kymaRepo error - nil returned",
			mrmRepo: &stubMrmRepo{
				mrm: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{Mandatory: mandatory},
				},
			},
			kymaRepo:  &stubKymaRepo{err: errors.New("kyma repo error")},
			wantCount: 0,
		},
		{
			name: "no kymas present - empty requests",
			mrmRepo: &stubMrmRepo{
				mrm: &v1beta2.ModuleReleaseMeta{
					Spec: v1beta2.ModuleReleaseMetaSpec{Mandatory: mandatory},
				},
			},
			kymaRepo:  &stubKymaRepo{kymaList: &v1beta2.KymaList{}},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := mrmwatch.NewMandatoryMrmChangeHandler(tt.mrmRepo, tt.kymaRepo)
			mapFunc := handler.Watch()
			obj := client.Object(&v1beta2.ModuleReleaseMeta{
				ObjectMeta: apimetav1.ObjectMeta{Name: "test-mrm"},
			})
			got := mapFunc(t.Context(), obj)
			require.Len(t, got, tt.wantCount)
			for _, req := range got {
				require.IsType(t, reconcile.Request{}, req)
			}
		})
	}
}
