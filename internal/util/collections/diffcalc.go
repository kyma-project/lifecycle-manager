package collections

// DiffCalc calculates the difference between two slices of objects for which a string-based identity can be defined.
// The provided identity function should return the smallest unique string representation of the object.
// Note: For most types one can find a unique string identity,
// but for some types it might not be possible or practical: one example would be a built-in map[k]v type.
// In such cases, the DiffCalc type should not be used.
type DiffCalc[E any] struct {
	First    []E
	Identity func(E) string
}

// notExistingIn returns a slice of object that are present in the first slice but not in the second.
// The returned list is a slice of pointers to the objects in the first slice, to avoid copying the objects.
func (d *DiffCalc[E]) NotExistingIn(second []E) []*E {
	onlyInFirst := make([]*E, 0, len(d.First))

	// Prepare presentInSecond map for diff calculation.
	presentInSecond := make(map[string]struct{}, len(second))
	for i := range second {
		presentInSecond[d.Identity(second[i])] = struct{}{}
	}
	// Calculate diff
	for i := range d.First {
		if _, isInSecond := presentInSecond[d.Identity(d.First[i])]; !isInSecond {
			onlyInFirst = append(onlyInFirst, &(d.First[i]))
		}
	}
	return onlyInFirst
}
