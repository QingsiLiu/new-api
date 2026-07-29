package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsAddsOnlyCreditsProjectionWhenEnabled(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	logs := []*Log{{
		Quota:       11_000,
		ChannelName: "private-channel",
		Other:       `{"admin_info":{"secret":"hidden"},"customer":"visible"}`,
	}}

	formatUserLogs(logs, 0)

	require.Equal(t, "3.055556", logs[0].Credits)
	require.Equal(t, common.CreditsQuotaUnit, logs[0].QuotaPerCredit)
	require.Empty(t, logs[0].ChannelName)
	require.NotContains(t, logs[0].Other, "admin_info")
	require.Contains(t, logs[0].Other, "customer")
	payload, err := common.Marshal(logs[0])
	require.NoError(t, err)
	require.Contains(t, string(payload), `"credits":"3.055556"`)
	require.Contains(t, string(payload), `"quota_per_credit":3600`)
	require.NotContains(t, string(payload), "admin_info")
	require.NotContains(t, string(payload), "private-channel")
}

func TestFormatUserLogsLeavesLegacyShapeWhenCreditsDisabled(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	logs := []*Log{{Quota: 7_200}}

	formatUserLogs(logs, 0)

	require.Empty(t, logs[0].Credits)
	require.Zero(t, logs[0].QuotaPerCredit)
	payload, err := common.Marshal(logs[0])
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"credits"`)
	require.NotContains(t, string(payload), `"quota_per_credit"`)
}
