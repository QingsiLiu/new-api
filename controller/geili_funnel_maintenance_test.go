package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestFunnelMaintenanceHandlerConfiguration(t *testing.T) {
	handler := funnelMaintenanceHandler{}
	require.Equal(t, model.SystemTaskTypeFunnelMaintenance, handler.Type())
	require.Equal(t, 24*time.Hour, handler.Interval())
	require.Nil(t, handler.NewPayload())

	t.Setenv("GEILI_FUNNEL_ENABLED", "false")
	t.Setenv("GEILI_FUNNEL_SECRET", "")
	require.False(t, handler.Enabled())
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", "short")
	require.False(t, handler.Enabled())
	t.Setenv("GEILI_FUNNEL_SECRET", strings.Repeat("s", 32))
	require.True(t, handler.Enabled())
}

func TestFunnelMaintenanceHandlerPersistsAggregateSuccess(t *testing.T) {
	setupFunnelControllerTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	task, runnerID := claimFunnelMaintenanceTask(t)

	funnelMaintenanceHandler{}.Run(context.Background(), task, runnerID)

	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finished)
	require.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
	var result model.FunnelMaintenanceResult
	require.NoError(t, common.UnmarshalJsonStr(finished.Result, &result))
	require.Positive(t, result.RawCutoff)
	require.Positive(t, result.ActivityCutoff)
	for _, forbidden := range []string{"visitor_hmac", "user_id", "cookie"} {
		require.NotContains(t, finished.Result, forbidden)
	}
}

func TestFunnelMaintenanceHandlerPersistsTerminalFailure(t *testing.T) {
	setupFunnelControllerTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	task, runnerID := claimFunnelMaintenanceTask(t)
	require.NoError(t, model.DB.Migrator().DropTable(&model.FunnelEvent{}))

	funnelMaintenanceHandler{}.Run(context.Background(), task, runnerID)

	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finished)
	require.Equal(t, model.SystemTaskStatusFailed, finished.Status)
	require.Empty(t, finished.Result)
	require.NotEmpty(t, finished.Error)
}

func TestFunnelMaintenanceHandlerHonorsCanceledLeaseContext(t *testing.T) {
	setupFunnelControllerTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	task, runnerID := claimFunnelMaintenanceTask(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	funnelMaintenanceHandler{}.Run(ctx, task, runnerID)

	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finished)
	require.Equal(t, model.SystemTaskStatusFailed, finished.Status)
	require.Contains(t, finished.Error, context.Canceled.Error())
}

func claimFunnelMaintenanceTask(t *testing.T) (*model.SystemTask, string) {
	t.Helper()
	task, err := model.CreateSystemTask(model.SystemTaskTypeFunnelMaintenance, nil, nil)
	require.NoError(t, err)
	runnerID := "funnel-maintenance-test"
	claimed, ok, err := model.ClaimSystemTask(task.ID, task.Type, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	return claimed, runnerID
}
