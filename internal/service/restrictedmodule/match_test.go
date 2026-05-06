package restrictedmodule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	restrictedmodulesvc "github.com/kyma-project/lifecycle-manager/internal/service/restrictedmodule"
)

func TestRestrictedModuleMatch_NilModuleReleaseMeta(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(nil, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_NilKyma(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{Spec: v1beta2.ModuleReleaseMetaSpec{}}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, nil)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_NilSelector(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{Spec: v1beta2.ModuleReleaseMetaSpec{KymaSelector: nil}}
	kyma := &v1beta2.Kyma{}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_EmptySelector(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{KymaSelector: &apimetav1.LabelSelector{}},
	}
	kyma := &v1beta2.Kyma{}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_NilLabelsOnKyma(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchLabels: map[string]string{"env": "production"},
			},
		},
	}
	kyma := &v1beta2.Kyma{}
	kyma.SetLabels(nil)

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_MatchLabels_Matches(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchLabels: map[string]string{"env": "production"},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"foo": "val1", "env": "production"}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.True(t, matched)
}

func TestRestrictedModuleMatch_MatchLabels_NoMatch(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchLabels: map[string]string{"env": "production"},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"foo": "val2", "env": "staging"}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_MatchExpressions_Matches(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchExpressions: []apimetav1.LabelSelectorRequirement{
					{Key: "tier", Operator: apimetav1.LabelSelectorOpIn, Values: []string{"frontend", "backend"}},
				},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"foo": "val3", "tier": "backend"}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.True(t, matched)
}

func TestRestrictedModuleMatch_MatchExpressions_NoMatch(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchExpressions: []apimetav1.LabelSelectorRequirement{
					{Key: "tier", Operator: apimetav1.LabelSelectorOpIn, Values: []string{"frontend", "backend"}},
				},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"tier": "database"}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_MatchLabelsAndExpressions_BothMatch(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchLabels: map[string]string{"env": "production"},
				MatchExpressions: []apimetav1.LabelSelectorRequirement{
					{Key: "tier", Operator: apimetav1.LabelSelectorOpIn, Values: []string{"frontend", "backend"}},
				},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{
			"foo":  "val4",
			"env":  "production",
			"tier": "backend",
		}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.True(t, matched)
}

func TestRestrictedModuleMatch_MatchLabelsAndExpressions_LabelMatchesExpressionDoesNot(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchLabels: map[string]string{"env": "production"},
				MatchExpressions: []apimetav1.LabelSelectorRequirement{
					{Key: "tier", Operator: apimetav1.LabelSelectorOpIn, Values: []string{"frontend", "backend"}},
				},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"env": "production", "tier": "database"}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_MatchLabelsAndExpressions_ExpressionMatchesLabelsDoNot(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchLabels: map[string]string{"env": "production"},
				MatchExpressions: []apimetav1.LabelSelectorRequirement{
					{Key: "tier", Operator: apimetav1.LabelSelectorOpIn, Values: []string{"frontend", "backend"}},
				},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"env": "staging", "tier": "backend"}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	require.NoError(t, err)
	assert.False(t, matched)
}

func TestRestrictedModuleMatch_MatchLabelsAndExpressions_ParseError(t *testing.T) {
	mrm := &v1beta2.ModuleReleaseMeta{
		Spec: v1beta2.ModuleReleaseMetaSpec{
			KymaSelector: &apimetav1.LabelSelector{
				MatchExpressions: []apimetav1.LabelSelectorRequirement{
					{Key: "tier", Operator: "InvalidOperator", Values: []string{"frontend", "backend"}},
				},
			},
		},
	}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"env": "production", "tier": "backend"}},
	}

	matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)

	assert.False(t, matched)
	require.Error(t, err)
	require.ErrorIs(t, err, restrictedmodulesvc.ErrSelectorParse)
	require.ErrorContains(t, err, "InvalidOperator")
	require.ErrorContains(t, err, "operator")
}
