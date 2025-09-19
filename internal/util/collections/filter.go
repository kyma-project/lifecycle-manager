package collections

// FilterInPlace modifies a slice using provided predicate function.
// It returns a sub-slice of the input list that only contains the elements
// for which the predicate function returns true.
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

// Filter returns a new slice which results from applying the provided predicate to the input slice.
func Filter[E any](input []E, predicate func(E) bool) []E {
	output := []E{}
	for _, val := range input {
		if predicate(val) {
			output = append(output, val)
		}
	}
	return output
}

// Dereference is a function that dereferences elements of a provided slice of pointers.
func Dereference[E any](list []*E) []E {
	res := make([]E, len(list))
	for i := range list {
		res[i] = *list[i]
	}
	return res
}
