package collections_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

func TestObjLists_diff_bothEmpty(t *testing.T) {
	// given
	first := fakeK8sObjectList{}
	assert.Empty(t, first, 0)

	second := fakeK8sObjectList{}
	assert.Empty(t, second, 0)

	// when
	diff := allFakeK8sObjectsIn(first).NotExistingIn(second)

	// then
	assert.Empty(t, diff)
}

func TestObjLists_diff_firstEmpty(t *testing.T) {
	// given
	first := fakeK8sObjectList{}
	second := fakeK8sObjectList{}.
		add("name1", "namespaceX").
		add("name2", "namespaceY")
	assert.Len(t, second, 2)

	// when
	diff := allFakeK8sObjectsIn(first).NotExistingIn(second)

	// then
	assert.Empty(t, diff, 0)
}

func TestObjLists_diff_secondEmpty(t *testing.T) {
	// given
	first := fakeK8sObjectList{}.
		add("name1", "namespace").
		add("name2", "namespace")
	second := fakeK8sObjectList{}

	// when
	diff := allFakeK8sObjectsIn(first).NotExistingIn(second)

	// then
	assert.Len(t, diff, 2)
	assert.Equal(t, first[0], *diff[0], first[0].GetName()+" should be in diff")
	assert.Equal(t, first[1], *diff[1], first[1].GetName()+" should be in diff")
}

func TestObjLists_diff_disjointLists(t *testing.T) {
	// given
	first := fakeK8sObjectList{}.
		add("name1", "namespace").
		add("name2", "namespace")
	second := fakeK8sObjectList{}.
		add("name3", "namespace").
		add("name4", "namespace")

	// when
	diff := allFakeK8sObjectsIn(first).NotExistingIn(second)

	// then
	assert.Len(t, diff, 2)
	assert.Contains(t, diff, &first[0])
	assert.Contains(t, diff, &first[1])
}

func TestObjLists_diff_overlappingLists(t *testing.T) {
	// given
	first := fakeK8sObjectList{}.
		add("name4", "namespace").
		add("name3", "namespace").
		add("name1", "namespace").
		add("name2", "namespace")
	assert.Len(t, first, 4)
	second := fakeK8sObjectList{}.
		add("name3", "namespace").
		add("name5", "namespace").
		add("name2", "namespace")
	assert.Len(t, second, 3)

	// when
	diff := allFakeK8sObjectsIn(first).NotExistingIn(second)

	// then
	assert.Len(t, diff, 2)
	assert.Contains(t, diff, &first[0])
	assert.Contains(t, diff, &first[2])
}

// In the production code this would be some existing K8s type, e.g. v1beta2.ModuleTemplate.
type fakeK8sObject struct {
	name      string
	namespace string
}

func (f fakeK8sObject) GetName() string {
	return f.name
}

func (f fakeK8sObject) GetNamespace() string {
	return f.namespace
}

// Convenience function for creating a DiffCalc with fakeK8sObject.
// Such an adapter is also recommended in the production code.
func allFakeK8sObjectsIn(fl []fakeK8sObject) *collections.DiffCalc[fakeK8sObject] {
	return &collections.DiffCalc[fakeK8sObject]{
		First: fl,
		Identity: func(obj fakeK8sObject) string {
			return obj.GetNamespace() + obj.GetName()
		},
	}
}

// Convenience type for creating fakeK8sObjectList in tests.
// This is not needed in the production code.
type fakeK8sObjectList []fakeK8sObject

func (f fakeK8sObjectList) add(nameArg, namespaceArg string) fakeK8sObjectList {
	f = append(f, fakeK8sObject{name: nameArg, namespace: namespaceArg})
	return f
}
