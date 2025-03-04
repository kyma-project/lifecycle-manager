package collections

// MergeMapsSilent merges map2 into map1.
// It does not indicate if map1 changed.
func MergeMapsSilent(map1, map2 map[string]string) map[string]string {
	mergedMap, _ := MergeMaps(map1, map2)
	return mergedMap
}

// MergeMaps merges map2 into map1.
// It returns true if map1 changed.
func MergeMaps(map1, map2 map[string]string) (map[string]string, bool) {
	changed := false
	if map1 == nil {
		map1 = make(map[string]string)
	}

	for k, v := range map2 {
		if map1[k] != v {
			map1[k] = v
			changed = true
		}
	}

	return map1, changed
}
