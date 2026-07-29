package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestTaskModel2DtoCreditsPreserveLegacyQuotaSemantics(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")

	tests := []struct {
		name            string
		status          model.TaskStatus
		wantState       string
		wantReserved    *int
		wantSettled     *int
		wantSettledText string
	}{
		{name: "running reservation", status: model.TaskStatusInProgress, wantState: "reserved", wantReserved: intPtr(11_000)},
		{name: "successful settlement", status: model.TaskStatusSuccess, wantState: "settled", wantSettled: intPtr(11_000), wantSettledText: "3.055556"},
		{name: "failed refund request", status: model.TaskStatusFailure, wantState: "refund_requested", wantReserved: intPtr(11_000), wantSettled: intPtr(0), wantSettledText: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &model.Task{TaskID: "task_contract", Status: tt.status, Quota: 11_000}
			got := TaskModel2Dto(task)

			require.Equal(t, 11_000, got.Quota, "legacy quota must never be redefined")
			require.NotNil(t, got.Credits)
			require.Equal(t, "3.055556", *got.Credits)
			require.NotNil(t, got.QuotaPerCredit)
			require.Equal(t, common.CreditsQuotaUnit, *got.QuotaPerCredit)
			require.Equal(t, tt.wantReserved, got.ReservedQuota)
			require.Equal(t, tt.wantSettled, got.SettledQuota)
			require.Equal(t, tt.wantState, got.BillingState)

			if tt.wantReserved != nil {
				require.NotNil(t, got.ReservedCredits)
				require.Equal(t, common.QuotaToCreditsString(*tt.wantReserved), *got.ReservedCredits)
			} else {
				require.Nil(t, got.ReservedCredits)
			}
			if tt.wantSettled != nil {
				require.NotNil(t, got.SettledCredits)
				require.Equal(t, tt.wantSettledText, *got.SettledCredits)
			} else {
				require.Nil(t, got.SettledCredits)
			}
		})
	}
}

func TestTaskModel2DtoCreditsAreAdditiveAndSerializeExplicitZero(t *testing.T) {
	task := &model.Task{TaskID: "task_failed", Status: model.TaskStatusFailure, Quota: 7_200}

	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	legacyJSON, err := common.Marshal(TaskModel2Dto(task))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id": 0,
		"created_at": 0,
		"updated_at": 0,
		"task_id": "task_failed",
		"platform": "",
		"user_id": 0,
		"group": "",
		"channel_id": 0,
		"quota": 7200,
		"action": "",
		"status": "FAILURE",
		"fail_reason": "",
		"submit_time": 0,
		"start_time": 0,
		"finish_time": 0,
		"progress": "",
		"properties": {"input": ""},
		"data": null
	}`, string(legacyJSON))

	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	creditsJSON, err := common.Marshal(TaskModel2Dto(task))
	require.NoError(t, err)
	require.Contains(t, string(creditsJSON), `"quota":7200`)
	require.Contains(t, string(creditsJSON), `"credits":"2"`)
	require.Contains(t, string(creditsJSON), `"reserved_quota":7200`)
	require.Contains(t, string(creditsJSON), `"reserved_credits":"2"`)
	require.Contains(t, string(creditsJSON), `"settled_quota":0`)
	require.Contains(t, string(creditsJSON), `"settled_credits":"0"`)
	require.Contains(t, string(creditsJSON), `"billing_state":"refund_requested"`)
}

func intPtr(value int) *int {
	return &value
}
