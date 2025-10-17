package templatelookup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func Test_GetOCMIdentity(t *testing.T) {
	t.Run("When ComponentIdentity is nil, then an error is returned", func(t *testing.T) {
		mtInfo := templatelookup.ModuleTemplateInfo{
			ModuleTemplate: &v1beta2.ModuleTemplate{},
			ComponentId:    nil,
		}
		_, err := mtInfo.GetOCMIdentity()
		require.Error(t, err)
		require.ErrorIs(t, err, templatelookup.ErrNoIdentity)
		assert.Contains(t, err.Error(), "component identity is nil")
	})
	t.Run("When ComponentIdentity is not nil, then it is returned", func(t *testing.T) {
		mtInfo := templatelookup.ModuleTemplateInfo{
			ComponentId: testutils.MustNewComponentId(testutils.FullOCMName("test-module"), "1.0.0"),
		}
		comp, err := mtInfo.GetOCMIdentity()
		require.NoError(t, err)
		assert.Equal(t, "kyma-project.io/module/test-module", comp.Name())
		assert.Equal(t, "1.0.0", comp.Version())
	})
}
