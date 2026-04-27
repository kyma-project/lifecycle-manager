package modulereleasemeta_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	mrmwatch "github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta"
)

type stubKymaRepo struct {
	kymaList *v1beta2.KymaList
	err      error
}

func (s *stubKymaRepo) LookupByLabel(_ context.Context, _, _ string) (*v1beta2.KymaList, error) {
	return s.kymaList, s.err
}

type fakeQueue struct {
	items      []reconcile.Request
	afterItems []reconcile.Request
}

func (q *fakeQueue) Add(item reconcile.Request) { q.items = append(q.items, item) }
func (q *fakeQueue) AddAfter(item reconcile.Request, _ time.Duration) {
	q.afterItems = append(q.afterItems, item)
}
func (q *fakeQueue) AddRateLimited(_ reconcile.Request)  {}
func (q *fakeQueue) Forget(_ reconcile.Request)          {}
func (q *fakeQueue) NumRequeues(_ reconcile.Request) int { return 0 }
func (q *fakeQueue) Len() int                            { return len(q.items) }
func (q *fakeQueue) Get() (reconcile.Request, bool)      { return reconcile.Request{}, false }
func (q *fakeQueue) Done(_ reconcile.Request)            {}
func (q *fakeQueue) ShutDown()                           {}
func (q *fakeQueue) ShutDownWithDrain()                  {}
func (q *fakeQueue) ShuttingDown() bool                  { return false }

func (q *fakeQueue) all() []reconcile.Request {
	return append(q.items, q.afterItems...)
}

var _ workqueue.TypedRateLimitingInterface[reconcile.Request] = (*fakeQueue)(nil)

func newMrmWithRegularChannel(version string) *v1beta2.ModuleReleaseMeta {
	return &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			ModuleName: "module",
			Channels:   []v1beta2.ChannelVersionAssignment{{Channel: "regular", Version: version}},
		},
	}
}

func kymaListWith(names ...string) *v1beta2.KymaList {
	items := make([]v1beta2.Kyma, 0, len(names))
	for _, name := range names {
		items = append(items, v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{Name: name, Namespace: "kcp-system"},
			Status: v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{{Name: "module", Channel: "regular"}},
			},
		})
	}
	return &v1beta2.KymaList{Items: items}
}

var errRepo = errors.New("repo error")

func TestTypedEventHandler_Delete(t *testing.T) {
	tests := []struct {
		name      string
		mrm       *v1beta2.ModuleReleaseMeta
		kymaRepo  *stubKymaRepo
		wantCount int
	}{
		{
			name:      "mrm deleted with matching kyma - kyma enqueued",
			mrm:       newMrmWithRegularChannel("1.0.0"),
			kymaRepo:  &stubKymaRepo{kymaList: kymaListWith("kyma-1")},
			wantCount: 1,
		},
		{
			name: "mrm with multiple channels - all matching kymas enqueued",
			mrm: &v1beta2.ModuleReleaseMeta{
				Spec: v1beta2.ModuleReleaseMetaSpec{
					ModuleName: "module",
					Channels: []v1beta2.ChannelVersionAssignment{
						{Channel: "regular", Version: "1.0.0"},
						{Channel: "fast", Version: "2.0.0"},
					},
				},
			},
			kymaRepo:  &stubKymaRepo{kymaList: kymaListWith("kyma-1")},
			wantCount: 1,
		},
		{
			name:      "repo error - nothing enqueued",
			mrm:       newMrmWithRegularChannel("1.0.0"),
			kymaRepo:  &stubKymaRepo{err: errRepo},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mrmwatch.NewEventHandler(tt.kymaRepo, 0)
			q := &fakeQueue{}
			h.Delete(t.Context(), event.DeleteEvent{Object: tt.mrm}, q)
			require.Len(t, q.all(), tt.wantCount)
		})
	}
}

func TestTypedEventHandler_Delete_NonMRMObject_IsNoop(t *testing.T) {
	h := mrmwatch.NewEventHandler(&stubKymaRepo{kymaList: kymaListWith("kyma-1")}, 0)
	q := &fakeQueue{}
	h.Delete(t.Context(), event.DeleteEvent{
		Object: &v1beta2.Kyma{ObjectMeta: apimetav1.ObjectMeta{Name: "some-kyma"}},
	}, q)
	require.Empty(t, q.all())
}

func TestTypedEventHandler_Update(t *testing.T) {
	tests := []struct {
		name      string
		oldMRM    *v1beta2.ModuleReleaseMeta
		newMRM    *v1beta2.ModuleReleaseMeta
		kymaRepo  *stubKymaRepo
		wantCount int
	}{
		{
			name:      "version changed - kyma enqueued",
			oldMRM:    newMrmWithRegularChannel("1.0.0"),
			newMRM:    newMrmWithRegularChannel("1.1.0"),
			kymaRepo:  &stubKymaRepo{kymaList: kymaListWith("kyma-1")},
			wantCount: 1,
		},
		{
			name:      "no version change - nothing enqueued",
			oldMRM:    newMrmWithRegularChannel("1.0.0"),
			newMRM:    newMrmWithRegularChannel("1.0.0"),
			kymaRepo:  &stubKymaRepo{kymaList: kymaListWith("kyma-1")},
			wantCount: 0,
		},
		{
			name:      "repo error - nothing enqueued",
			oldMRM:    newMrmWithRegularChannel("1.0.0"),
			newMRM:    newMrmWithRegularChannel("1.1.0"),
			kymaRepo:  &stubKymaRepo{err: errRepo},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mrmwatch.NewEventHandler(tt.kymaRepo, 0)
			q := &fakeQueue{}
			h.Update(t.Context(), event.UpdateEvent{ObjectOld: tt.oldMRM, ObjectNew: tt.newMRM}, q)
			require.Len(t, q.all(), tt.wantCount)
		})
	}
}

func TestTypedEventHandler_Update_NonMRMObject_IsNoop(t *testing.T) {
	h := mrmwatch.NewEventHandler(&stubKymaRepo{kymaList: kymaListWith("kyma-1")}, 0)
	q := &fakeQueue{}
	h.Update(t.Context(), event.UpdateEvent{
		ObjectOld: &v1beta2.Kyma{ObjectMeta: apimetav1.ObjectMeta{Name: "kyma"}},
		ObjectNew: newMrmWithRegularChannel("1.1.0"),
	}, q)
	require.Empty(t, q.all())
}

func TestTypedEventHandler_Update_WithDelay_UsesAddAfter(t *testing.T) {
	h := mrmwatch.NewEventHandler(&stubKymaRepo{kymaList: kymaListWith("kyma-1")}, 10*time.Second)
	q := &fakeQueue{}
	h.Update(t.Context(), event.UpdateEvent{
		ObjectOld: newMrmWithRegularChannel("1.0.0"),
		ObjectNew: newMrmWithRegularChannel("1.1.0"),
	}, q)
	require.Len(t, q.afterItems, 1)
	require.Empty(t, q.items)
}

func TestTypedEventHandler_Create_IsNoop(t *testing.T) {
	h := mrmwatch.NewEventHandler(&stubKymaRepo{kymaList: kymaListWith("kyma-1")}, 0)
	q := &fakeQueue{}
	h.Create(t.Context(), event.CreateEvent{Object: newMrmWithRegularChannel("1.0.0")}, q)
	require.Empty(t, q.all())
}

func TestTypedEventHandler_Generic_IsNoop(t *testing.T) {
	h := mrmwatch.NewEventHandler(&stubKymaRepo{kymaList: kymaListWith("kyma-1")}, 0)
	q := &fakeQueue{}
	h.Generic(t.Context(), event.GenericEvent{Object: newMrmWithRegularChannel("1.0.0")}, q)
	require.Empty(t, q.all())
}
