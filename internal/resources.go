package internal

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// ManifestResources holds a collection of objects, so that we can filter / sequence them.
type ManifestResources struct {
	Items []*unstructured.Unstructured
	Blobs [][]byte
}
