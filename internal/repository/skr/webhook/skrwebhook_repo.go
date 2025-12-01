package webhook

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type ResourceRepository struct {
	resources []*unstructured.Unstructured
}

func NewResourceRepository(webhookResources []*unstructured.Unstructured) *ResourceRepository {
	return &ResourceRepository{
		resources: webhookResources,
	}
}
