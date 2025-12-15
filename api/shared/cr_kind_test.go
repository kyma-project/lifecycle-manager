package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

func TestKind_Plural(t *testing.T) {
	kymaKind := shared.KymaKind

	kymaPlural := kymaKind.Plural()

	assert.Equal(t, "kymas", kymaPlural)
}

func TestKind_List(t *testing.T) {
	kymaKind := shared.KymaKind

	kymaList := kymaKind.List()

	assert.Equal(t, "KymaList", kymaList)
}
