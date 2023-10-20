package v2

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate mockgen -source object.go -destination mock/object.go Object
type Object interface {
	client.Object
	GetStatus() shared.Status
	SetStatus(shared.Status)
}
