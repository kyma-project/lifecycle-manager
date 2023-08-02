package types

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LabelNotFoundError struct {
	Resource  client.Object
	LabelName string
}

func (m *LabelNotFoundError) Error() string {
	return fmt.Sprintf("label %s not found on resource %s", m.LabelName,
		client.ObjectKeyFromObject(m.Resource).String())
}
