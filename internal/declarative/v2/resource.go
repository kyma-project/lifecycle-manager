package v2

import "k8s.io/cli-runtime/pkg/resource"

// ResourceList provides convenience methods for comparing collections of Infos. Copy from helm.sh/helm/v3/pkg/kube.
type ResourceList []*resource.Info

// Append adds an Info to the Result.
func (r *ResourceList) Append(val *resource.Info) {
	*r = append(*r, val)
}

// Visit implements resource.Visitor.
func (r ResourceList) Visit(fn resource.VisitorFunc) error {
	for _, i := range r {
		if err := fn(i, nil); err != nil {
			return err
		}
	}
	return nil
}

// Filter returns a new Result with Infos that satisfy the predicate fn.
func (r ResourceList) Filter(fn func(*resource.Info) bool) ResourceList {
	var result ResourceList
	for _, i := range r {
		if fn(i) {
			result.Append(i)
		}
	}
	return result
}

// Get returns the Info from the result that matches the name and kind.
func (r ResourceList) Get(info *resource.Info) *resource.Info {
	for _, i := range r {
		if isMatchingInfo(i, info) {
			return i
		}
	}
	return nil
}

// Contains checks to see if an object exists.
func (r ResourceList) Contains(info *resource.Info) bool {
	for _, i := range r {
		if isMatchingInfo(i, info) {
			return true
		}
	}
	return false
}

// Difference will return a new Result with objects not contained in rs.
func (r ResourceList) Difference(rs ResourceList) ResourceList {
	return r.Filter(func(info *resource.Info) bool {
		return !rs.Contains(info)
	})
}

// Intersect will return a new Result with objects contained in both Results.
func (r ResourceList) Intersect(rs ResourceList) ResourceList {
	return r.Filter(rs.Contains)
}

// isMatchingInfo returns true if infos match on Name and GroupVersionKind.
func isMatchingInfo(a, b *resource.Info) bool {
	return a.Name == b.Name && a.Namespace == b.Namespace &&
		a.Mapping.GroupVersionKind.Kind == b.Mapping.GroupVersionKind.Kind
}
