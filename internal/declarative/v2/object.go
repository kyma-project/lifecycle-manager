package v2

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

//go:generate mockgen -source object.go -destination mock/object.go Object
type Object interface {
	client.Object
	GetStatus() shared.Status
	SetStatus(status shared.Status)
}
