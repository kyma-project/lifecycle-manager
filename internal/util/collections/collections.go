package collections

func MergeMaps(map1, map2 map[string]string) map[string]string {
	mergedMap := make(map[string]string)

	for k, v := range map1 {
		mergedMap[k] = v
	}

	for k, v := range map2 {
		mergedMap[k] = v
	}

	return mergedMap
}
