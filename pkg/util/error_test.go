package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

func TestIsTLSCertExpiredError(t *testing.T) {
	errorMessageFromLogs := "couldn't get current server API group list: " +
		"Get \"https://api.somehost.dev.kyma.com/api?timeout=32s\": remote error: tls: expired certificate"
	assert.True(t, util.IsTLSCertExpiredError(errorMessageFromLogs))
}

func TestIsUnauthorizedError(t *testing.T) {
	errorMessageFromLogs := "patch for clusterrolebindings/template-operator-proxy-rolebinding failed: Unauthorized\n"
	assert.True(t, util.IsUnauthorizedError(errorMessageFromLogs))
}
