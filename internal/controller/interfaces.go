package controller

import (
	"github.com/kyma-project/lifecycle-manager/internal/common"
)

// EventService re-exports the common EventService interface for backward compatibility.
// This allows controllers to use controller.EventService while avoiding circular dependencies.
type EventService = common.EventService
