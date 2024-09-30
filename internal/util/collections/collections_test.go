package collections_test

import (
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMergeMaps_WhenMapIsEmpty(t *testing.T) {
	firstMap := map[string]string{}
	secondMap := map[string]string{
		"key": "value",
	}

	mergedMap := collections.MergeMaps(firstMap, secondMap)
	require.Len(t, mergedMap, 1)
	require.Contains(t, mergedMap, "key")
	require.Equal(t, "value", mergedMap["key"])
}

func TestMergeMaps_WhenMapIsNil(t *testing.T) {
	var firstMap map[string]string
	secondMap := map[string]string{
		"key": "value",
	}

	mergedMap := collections.MergeMaps(firstMap, secondMap)
	require.Len(t, mergedMap, 1)
	require.Contains(t, mergedMap, "key")
	require.Equal(t, "value", mergedMap["key"])
}

func TestMergeMaps_WhenBothMapsHaveValues(t *testing.T) {
	firstMap := map[string]string{
		"key1": "value1",
	}
	secondMap := map[string]string{
		"key2": "value2",
	}

	mergedMap := collections.MergeMaps(firstMap, secondMap)
	require.Len(t, mergedMap, 2)
	require.Contains(t, mergedMap, "key1")
	require.Equal(t, "value1", mergedMap["key1"])
	require.Contains(t, mergedMap, "key2")
	require.Equal(t, "value2", mergedMap["key2"])
}
