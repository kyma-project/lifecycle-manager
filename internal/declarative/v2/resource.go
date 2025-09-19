package v2

import "k8s.io/cli-runtime/pkg/resource"

// ResourceList provides convenience methods for comparing collections of Infos.
// Copy from https://github.com/helm/helm/blob/v3.19.0/pkg/kube/resource.go
type ResourceList []*resource.Info

// Difference will return a new Result with objects not contained in rs.
func (r ResourceList) Difference(rs ResourceList) ResourceList {
	return r.filter(func(info *resource.Info) bool {
		return !rs.contains(info)
	})
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

// append adds an Info to the Result.
func (r *ResourceList) append(val *resource.Info) {
	*r = append(*r, val)
}

// filter returns a new Result with Infos that satisfy the predicate fn.
func (r ResourceList) filter(fn func(*resource.Info) bool) ResourceList {
	var result ResourceList
	for _, i := range r {
		if fn(i) {
			result.append(i)
		}
	}
	return result
}

// contains checks to see if an object exists.
func (r ResourceList) contains(info *resource.Info) bool {
	for _, i := range r {
		if isMatchingInfo(i, info) {
			return true
		}
	}
	return false
}

// isMatchingInfo returns true if infos match on Name and GroupVersionKind.
func isMatchingInfo(a, b *resource.Info) bool {
	return a.Name == b.Name && a.Namespace == b.Namespace &&
		a.Mapping.GroupVersionKind.Kind == b.Mapping.GroupVersionKind.Kind
}
