package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestTasksToDtoProjectsCreditsLifecycle(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")

	items := tasksToDto([]*model.Task{
		{TaskID: "task_running", Quota: 7200, Status: model.TaskStatusInProgress},
		{TaskID: "task_success", Quota: 11000, Status: model.TaskStatusSuccess},
		{TaskID: "task_failure", Quota: 30000, Status: model.TaskStatusFailure},
	}, false)
	require.Len(t, items, 3)

	require.Equal(t, "2", items[0].Credits)
	require.Equal(t, common.CreditsQuotaUnit, items[0].QuotaPerCredit)
	require.NotNil(t, items[0].ReservedQuota)
	require.Equal(t, 7200, *items[0].ReservedQuota)
	require.Equal(t, "2", items[0].ReservedCredits)
	require.Nil(t, items[0].SettledQuota)
	require.Equal(t, "reserved", items[0].BillingState)

	require.Equal(t, "3.055556", items[1].Credits)
	require.Nil(t, items[1].ReservedQuota)
	require.NotNil(t, items[1].SettledQuota)
	require.Equal(t, 11000, *items[1].SettledQuota)
	require.Equal(t, "3.055556", items[1].SettledCredits)
	require.Equal(t, "settled", items[1].BillingState)

	require.Equal(t, "8.333333", items[2].Credits)
	require.NotNil(t, items[2].ReservedQuota)
	require.Equal(t, 30000, *items[2].ReservedQuota)
	require.Equal(t, "8.333333", items[2].ReservedCredits)
	require.NotNil(t, items[2].SettledQuota)
	require.Zero(t, *items[2].SettledQuota)
	require.Equal(t, "0", items[2].SettledCredits)
	require.Equal(t, "refund_requested", items[2].BillingState)
}

func TestTasksToDtoKeepsCreditsFieldsOffByDefault(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "false")

	item := tasksToDto([]*model.Task{{
		TaskID: "task_legacy",
		Quota:  7200,
		Status: model.TaskStatusSuccess,
	}}, false)[0]

	require.Empty(t, item.Credits)
	require.Zero(t, item.QuotaPerCredit)
	require.Nil(t, item.ReservedQuota)
	require.Nil(t, item.SettledQuota)
	require.Empty(t, item.BillingState)
}
