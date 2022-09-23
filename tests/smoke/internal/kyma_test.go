package internal

import (
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"os"
	"os/exec"
	"testing"
)

func TestSetupCLI(t *testing.T) {
	a := assert.New(t)
	a.NoError(SetupKymaCLI())
}

func TestKymaProvision(t *testing.T) {
	a := assert.New(t)
	log := zaptest.NewLogger(t)
	defer func(log *zap.SugaredLogger) {
		_ = log.Sync()
	}(log.Sugar())

	a.NoError(SetupKymaCLI())
	provisionK3dCommand := KymaCLI("provision", "k3d")
	a.NoError(provisionK3dCommand.Run())

	t.Cleanup(func() {
		deprovision := exec.Command("k3d", "cluster", "delete", "kyma")
		deprovision.Stdout = os.Stdout
		deprovision.Stderr = os.Stderr
		a.NoError(deprovision.Run())
	})
}
