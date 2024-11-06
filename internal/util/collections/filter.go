package collections

// filterInPlace is a function that filters a slice by a predicate function.
// It returns a sub-slice of the input list that only contains the elements for which the predicate function returns true.
// Warning: This function modifies the input list!
func FilterInPlace[E any](list []*E, predicate func(*E) bool) []*E {
	last := 0
	for i := range list {
		if predicate(list[i]) {
			list[last] = list[i]
			last++
		}
	}
	return list[:last]
}

// Dereference is a function that dereferences elements of a provided slice of pointers.
func Dereference[E any](list []*E) []E {
	res := make([]E, len(list))
	for i := range list {
		res[i] = *list[i]
	}
	return res
}
