package events_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta/events"
)

func kymaList(names ...string) *v1beta2.KymaList {
	items := make([]v1beta2.Kyma, 0, len(names))
	for _, name := range names {
		items = append(items, v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{Name: name, Namespace: "kcp-system"},
		})
	}
	return &v1beta2.KymaList{Items: items}
}

func mandatoryMrm() *v1beta2.ModuleReleaseMeta {
	return &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			ModuleName: "module",
			Mandatory:  &v1beta2.Mandatory{Version: "1.0.0"},
		},
	}
}

func regularMrm() *v1beta2.ModuleReleaseMeta {
	return &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			ModuleName: "module",
			Channels:   []v1beta2.ChannelVersionAssignment{{Channel: "regular", Version: "1.0.0"}},
		},
	}
}

func TestRegularResolver_OnCreate_ReturnsNothing(t *testing.T) {
	got := events.RegularResolver{}.OnCreate(regularMrm(), kymaList("kyma-1"))
	require.Empty(t, got)
}

func TestMandatoryResolver_ReturnsAllManagedKymas(t *testing.T) {
	kymas := kymaList("kyma-1", "kyma-2")
	want := []*types.NamespacedName{
		{Name: "kyma-1", Namespace: "kcp-system"},
		{Name: "kyma-2", Namespace: "kcp-system"},
	}
	resolver := events.MandatoryResolver{}

	require.Equal(t, want, resolver.OnCreate(mandatoryMrm(), kymas))
	require.Equal(t, want, resolver.OnUpdate(mandatoryMrm(), mandatoryMrm(), kymas))
	require.Equal(t, want, resolver.OnDelete(mandatoryMrm(), kymas))
}

func TestMandatoryResolver_NonMandatoryMrm_ReturnsNothing(t *testing.T) {
	kymas := kymaList("kyma-1")
	resolver := events.MandatoryResolver{}

	require.Empty(t, resolver.OnCreate(regularMrm(), kymas))
	require.Empty(t, resolver.OnUpdate(regularMrm(), regularMrm(), kymas))
	require.Empty(t, resolver.OnDelete(regularMrm(), kymas))
}
