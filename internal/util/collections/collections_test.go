package collections_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

func TestMergeMapsSilent_WhenMapIsEmpty(t *testing.T) {
	firstMap := map[string]string{}
	secondMap := map[string]string{
		"key": "value",
	}

	mergedMap := collections.MergeMapsSilent(firstMap, secondMap)

	require.Len(t, mergedMap, 1)
	require.Contains(t, mergedMap, "key")
	require.Equal(t, "value", mergedMap["key"])
}

func TestMergeMapsSilent_WhenMapIsNil(t *testing.T) {
	var firstMap map[string]string
	secondMap := map[string]string{
		"key": "value",
	}

	mergedMap := collections.MergeMapsSilent(firstMap, secondMap)

	require.Len(t, mergedMap, 1)
	require.Contains(t, mergedMap, "key")
	require.Equal(t, "value", mergedMap["key"])
}

func TestMergeMapsSilent_WhenBothMapsHaveValues(t *testing.T) {
	firstMap := map[string]string{
		"key1": "value1",
	}
	secondMap := map[string]string{
		"key2": "value2",
	}

	mergedMap := collections.MergeMapsSilent(firstMap, secondMap)

	require.Len(t, mergedMap, 2)
	require.Contains(t, mergedMap, "key1")
	require.Equal(t, "value1", mergedMap["key1"])
	require.Contains(t, mergedMap, "key2")
	require.Equal(t, "value2", mergedMap["key2"])
}

func TestMergeMaps_WhenMapIsNil(t *testing.T) {
	var firstMap map[string]string = nil
	secondMap := map[string]string{
		"key": "value",
	}

	mergedMap, changed := collections.MergeMaps(firstMap, secondMap)

	require.True(t, changed)
	require.Len(t, mergedMap, 1)
	for k, v := range secondMap {
		require.Equal(t, v, mergedMap[k])
	}
	require.Nil(t, firstMap)
}

func TestMergeMaps_WhenMapIsEmpty(t *testing.T) {
	firstMap := map[string]string{}
	secondMap := map[string]string{
		"key": "value",
	}

	mergedMap, changed := collections.MergeMaps(firstMap, secondMap)

	require.True(t, changed)
	require.Len(t, mergedMap, 1)
	for k, v := range secondMap {
		require.Equal(t, v, mergedMap[k])
	}
	for k, v := range firstMap {
		require.Equal(t, v, mergedMap[k])
	}
}

func TestMergeMaps_WhenSecondMapIsNil(t *testing.T) {
	firstMap := map[string]string{
		"key": "value",
	}
	var secondMap map[string]string = nil

	mergedMap, changed := collections.MergeMaps(firstMap, secondMap)

	require.False(t, changed)
	require.Len(t, mergedMap, 1)
	for k, v := range firstMap {
		require.Equal(t, v, mergedMap[k])
	}
	require.Nil(t, secondMap)
}

func TestMergeMaps_WhenEntriesDistinct(t *testing.T) {
	firstMap := map[string]string{
		"key1": "value1",
	}
	secondMap := map[string]string{
		"key2": "value2",
	}

	mergedMap, changed := collections.MergeMaps(firstMap, secondMap)

	require.Len(t, mergedMap, 2)
	require.True(t, changed)
	for k, v := range firstMap {
		require.Equal(t, v, mergedMap[k])
	}
	for k, v := range secondMap {
		require.Equal(t, v, mergedMap[k])
	}
}

func TestMergeMaps_WhenSecondMapOverwritingFirst(t *testing.T) {
	firstMap := map[string]string{
		"key1": "value1",
	}
	secondMap := map[string]string{
		"key1": "value2",
	}

	mergedMap, changed := collections.MergeMaps(firstMap, secondMap)

	require.Len(t, mergedMap, 1)
	require.True(t, changed)
	for k, v := range secondMap {
		require.Equal(t, v, mergedMap[k])
	}
}
