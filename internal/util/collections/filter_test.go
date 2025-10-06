package collections_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

func TestInPlaceFilter_empty(t *testing.T) {
	// given
	list := []*string{}
	predicate := func(*string) bool { return true }

	// when
	actual := collections.FilterInPlace(list, predicate)

	// then
	assert.Empty(t, actual)
}

func TestInPlaceFilter_predicate_always_matches(t *testing.T) {
	// given
	initial := listOfStringPtr("a", "b", "c")
	aCopy := append([]*string(nil), initial...) // we use the copy because the filterInPlace may modify the input list.
	predicate := func(*string) bool { return true }

	// when
	actual := collections.FilterInPlace(aCopy, predicate)

	// then
	assert.Len(t, actual, 3)
	assert.Equal(t, initial, actual)
}

func TestInPlaceFilter_predicate_never_matches(t *testing.T) {
	// given
	initial := listOfStringPtr("a", "b", "c")
	aCopy := append([]*string(nil), initial...) // we use the copy because the filterInPlace may modify the input list.
	predicate := func(*string) bool { return false }

	// when
	actual := collections.FilterInPlace(aCopy, predicate)

	// then
	assert.Empty(t, actual)
}

func TestInPlaceFilter_retains_order(t *testing.T) {
	// given
	initial := listOfStringPtr("a", "b", "c", "d", "e", "f", "g")
	aCopy := append([]*string(nil), initial...) // we use the copy because the filterInPlace may modify the input list.
	predicate := func(val *string) bool { return inRange(*val, "a", "g") && (*val != "d") }

	// when
	actual := collections.FilterInPlace(aCopy, predicate)

	// then
	assert.Len(t, actual, 4)
	assert.Equal(t, "b", *actual[0])
	assert.Equal(t, "c", *actual[1])
	assert.Equal(t, "e", *actual[2])
	assert.Equal(t, "f", *actual[3])

	// some technical checks about the modified "aCopy" list.
	assert.Len(t, aCopy, len(initial)) // the copy has the same length as the initial list.

	// Returned "actual" list is the same as the "head" of "aCopy" list.
	assert.Equal(t, actual[0], aCopy[0])
	assert.Equal(t, actual[1], aCopy[1])
	assert.Equal(t, actual[2], aCopy[2])
	assert.Equal(t, actual[3], aCopy[3])

	// The head of "aCopy" is modified after filtering.
	assert.NotEqual(t, initial[0], aCopy[0])
	assert.NotEqual(t, initial[1], aCopy[1])
	assert.NotEqual(t, initial[2], aCopy[2])
	assert.NotEqual(t, initial[3], aCopy[3])

	// The "tail" of aCopy is the same as the tail of the "initial" list.
	assert.Equal(t, initial[4], aCopy[4])
	assert.Equal(t, initial[5], aCopy[5])
	assert.Equal(t, initial[6], aCopy[6])
}

func listOfStringPtr(strings ...string) []*string {
	list := make([]*string, len(strings))
	for i := range strings {
		list[i] = &strings[i]
	}
	return list
}

func inRange(val, lowerBound, upperBound string) bool {
	return lowerBound < val && val < upperBound
}

func Test_Filter(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		// given
		list := []string{}
		predicate := func(string) bool { return true }

		// when
		actual := collections.Filter(list, predicate)

		// then
		assert.Empty(t, actual)
	})
	t.Run("predicate always true", func(t *testing.T) {
		// given
		list := []string{"a", "b", "c"}
		predicate := func(string) bool { return true }

		// when
		actual := collections.Filter(list, predicate)

		// then
		assert.Equal(t, list, actual)
	})
	t.Run("predicate always false", func(t *testing.T) {
		// given
		list := []string{"a", "b", "c"}
		predicate := func(string) bool { return false }

		// when
		actual := collections.Filter(list, predicate)

		// then
		assert.Empty(t, actual)
	})
	t.Run("predicate retains order", func(t *testing.T) {
		// given
		list := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
		predicate := func(val string) bool { return inRange(val, "b", "g") && (val != "d") }

		// when
		actual := collections.Filter(list, predicate)

		// then
		assert.Equal(t, []string{"c", "e", "f"}, actual)
	})
}
