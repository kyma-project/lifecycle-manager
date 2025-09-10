package oci

// compiled only when running tests.
func SetCraneWrapper(r *RepositoryReader, cWrap craneWrapper) {
	r.cWrapper = cWrap
}
