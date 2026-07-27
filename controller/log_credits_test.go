package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestUserLogStatCreditsFollowFeatureFlag(t *testing.T) {
	stat := model.Stat{Quota: 11_000, Rpm: 7, Tpm: 23}

	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	legacy := userLogStatData(stat)
	require.Equal(t, 11_000, legacy["quota"])
	require.Equal(t, 7, legacy["rpm"])
	require.Equal(t, 23, legacy["tpm"])
	require.NotContains(t, legacy, "credits")
	require.NotContains(t, legacy, "quota_per_credit")

	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	credits := userLogStatData(stat)
	require.Equal(t, "3.055556", credits["credits"])
	require.Equal(t, common.CreditsQuotaUnit, credits["quota_per_credit"])
	require.Equal(t, 11_000, credits["quota"])
}
