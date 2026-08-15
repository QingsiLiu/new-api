package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestGetGeiliFunnelHealthReturnsAggregateOnlyWarnings(t *testing.T) {
	setupFunnelServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	secret := strings.Repeat("s", 32)
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", secret)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&model.Option{
		Key: "GeiliFunnelCollectionStartedAt.production", Value: fmt.Sprintf("%d", now-10*3600),
	}).Error)
	linkedUser := 7
	linkedHash := strings.Repeat("a", 64)
	ambiguousHash := strings.Repeat("b", 64)
	require.NoError(t, model.DB.Create(&model.FunnelVisitor{
		Environment: model.FunnelEnvironmentProduction, VisitorHMAC: &linkedHash,
		IdentityState: model.FunnelIdentityLinked, UserID: &linkedUser, FirstSeenAt: now - 100, LastSeenAt: now - 10,
	}).Error)
	require.NoError(t, model.DB.Create(&model.FunnelVisitor{
		Environment: model.FunnelEnvironmentProduction, VisitorHMAC: &ambiguousHash,
		IdentityState: model.FunnelIdentityAmbiguous, FirstSeenAt: now - 100, LastSeenAt: now - 10,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{UserId: 7, TradeNo: "health-invalid", Status: common.TopUpStatusSuccess, CompleteTime: 0}).Error)
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "health-success", UserId: 7, Status: model.TaskStatusSuccess, FinishTime: now - 10}).Error)
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "health-failure", UserId: 7, Status: model.TaskStatusFailure, FinishTime: now - 10}).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: 7, Name: "production", Key: "health-key", CreatedTime: now - 10}).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: 8, Name: "invalid", Key: "health-invalid-key", CreatedTime: 0}).Error)
	require.NoError(t, model.DB.Create(&model.ModelRegistry{ModelName: "gpt-5.5", Slug: "gpt-5-5", Modality: "text"}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 7, Type: model.LogTypeConsume, CreatedAt: now - 10, ModelName: "gpt-5.5", PromptTokens: 1, RequestId: "health-text"}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 8, Type: model.LogTypeConsume, CreatedAt: -1, ModelName: "gpt-5.5", CompletionTokens: 1, RequestId: "health-invalid-text"}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 9, Type: model.LogTypeConsume, CreatedAt: now - 5, ModelName: "gpt-5.5", RequestId: "health-zero-token"}).Error)

	response, err := GetGeiliFunnelHealth(context.Background(), now, model.FunnelEnvironmentProduction)
	require.NoError(t, err)
	require.True(t, response.Healthy)
	require.Equal(t, "pending_initial_run", response.Maintenance.Status)
	require.Equal(t, 2, response.SchemaVersion)
	require.Len(t, response.Events, 5)
	require.EqualValues(t, 1, response.Identity.Linked)
	require.EqualValues(t, 1, response.Identity.Ambiguous)
	require.Equal(t, 0.5, *response.Identity.AmbiguousRate)
	require.EqualValues(t, 1, response.Business.InvalidTopUpTimes)
	require.EqualValues(t, 1, response.Business.InvalidTokenTimes)
	require.EqualValues(t, 1, response.Business.InvalidTextLogTimes)
	require.EqualValues(t, 1, response.Business.FirstAPIKeysLast24h)
	require.EqualValues(t, 1, response.Business.FirstActivatedLast24h)
	require.EqualValues(t, 1, response.Business.FirstSuccessfulTextLast24h)
	require.EqualValues(t, 1, response.Business.TaskSuccessLast24h)
	require.EqualValues(t, 1, response.Business.TaskFailureLast24h)

	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	for _, forbidden := range []string{"visitor_hmac", "user_id", "cookie", "authorization", "prompt", "url", "error_message", secret, linkedHash} {
		require.NotContains(t, string(encoded), forbidden)
	}
}

func TestGetGeiliFunnelHealthRequiresFreshConsistentMaintenance(t *testing.T) {
	setupFunnelServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", strings.Repeat("s", 32))
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&model.Option{
		Key: "GeiliFunnelCollectionStartedAt.production", Value: fmt.Sprintf("%d", now-40*3600),
	}).Error)

	stale, err := GetGeiliFunnelHealth(context.Background(), now, model.FunnelEnvironmentProduction)
	require.NoError(t, err)
	require.False(t, stale.Healthy)
	require.Equal(t, "stale", stale.Maintenance.Status)

	runAt := now - 3600
	task := finishHealthMaintenanceTask(t, model.SystemTaskStatusSucceeded, model.FunnelMaintenanceResult{
		RawCutoff: runAt - 180*86400, ActivityCutoff: utcDay(runAt) - 730*86400,
	})
	require.NoError(t, model.DB.Model(&model.SystemTask{}).Where("id = ?", task.ID).Update("updated_at", runAt).Error)
	healthy, err := GetGeiliFunnelHealth(context.Background(), now, model.FunnelEnvironmentProduction)
	require.NoError(t, err)
	require.True(t, healthy.Healthy)
	require.Equal(t, "succeeded", healthy.Maintenance.Status)
	require.EqualValues(t, runAt, healthy.Maintenance.LastSuccessfulAt)

	finishHealthMaintenanceTask(t, model.SystemTaskStatusFailed, nil)
	failed, err := GetGeiliFunnelHealth(context.Background(), now, model.FunnelEnvironmentProduction)
	require.NoError(t, err)
	require.False(t, failed.Healthy)
	require.Equal(t, "failed", failed.Maintenance.Status)
	encoded, err := json.Marshal(failed)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "dynamic private database error")
}

func TestApplyFunnelMaintenanceHealthRejectsStaleOrInconsistentResults(t *testing.T) {
	now := int64(2_000_000_000)
	startedAt := now - 40*3600
	validResult := model.FunnelMaintenanceResult{
		RawCutoff:      now - 180*86400,
		ActivityCutoff: utcDay(now) - 730*86400,
	}
	validJSON, err := common.Marshal(validResult)
	require.NoError(t, err)
	staleRunAt := now - 37*3600
	staleResult := model.FunnelMaintenanceResult{
		RawCutoff:      staleRunAt - 180*86400,
		ActivityCutoff: utcDay(staleRunAt) - 730*86400,
	}
	staleJSON, err := common.Marshal(staleResult)
	require.NoError(t, err)

	staleSuccess := &model.SystemTask{
		ID: 1, Type: model.SystemTaskTypeFunnelMaintenance, Status: model.SystemTaskStatusSucceeded,
		Result: string(staleJSON), UpdatedAt: staleRunAt,
	}
	response := dto.GeiliFunnelHealthResponse{CollectionStartedAt: startedAt}
	applyFunnelMaintenanceHealth(&response, now, staleSuccess, staleSuccess)
	require.False(t, response.Healthy)
	require.Equal(t, "stale", response.Maintenance.Status)

	inconsistentResult := validResult
	inconsistentResult.ActivityCutoff++
	inconsistentJSON, err := common.Marshal(inconsistentResult)
	require.NoError(t, err)
	inconsistent := &model.SystemTask{
		ID: 2, Type: model.SystemTaskTypeFunnelMaintenance, Status: model.SystemTaskStatusSucceeded,
		Result: string(inconsistentJSON), UpdatedAt: now,
	}
	response = dto.GeiliFunnelHealthResponse{CollectionStartedAt: startedAt}
	applyFunnelMaintenanceHealth(&response, now, inconsistent, inconsistent)
	require.False(t, response.Healthy)
	require.Equal(t, "stale", response.Maintenance.Status)

	pending := &model.SystemTask{ID: 3, Type: model.SystemTaskTypeFunnelMaintenance, Status: model.SystemTaskStatusPending, UpdatedAt: now}
	fresh := &model.SystemTask{
		ID: 2, Type: model.SystemTaskTypeFunnelMaintenance, Status: model.SystemTaskStatusSucceeded,
		Result: string(validJSON), UpdatedAt: now,
	}
	response = dto.GeiliFunnelHealthResponse{CollectionStartedAt: startedAt}
	applyFunnelMaintenanceHealth(&response, now, pending, fresh)
	require.True(t, response.Healthy)
	require.Equal(t, "pending", response.Maintenance.Status)
}

func TestGetGeiliFunnelHealthFailsClosedOnSchemaOrConfig(t *testing.T) {
	setupFunnelServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", "short")
	_, err := GetGeiliFunnelHealth(context.Background(), common.GetTimestamp(), model.FunnelEnvironmentProduction)
	require.ErrorIs(t, err, ErrGeiliFunnelHealthUnavailable)

	t.Setenv("GEILI_FUNNEL_SECRET", strings.Repeat("s", 32))
	require.NoError(t, model.DB.Migrator().DropTable(&model.FunnelActivityDay{}))
	_, err = GetGeiliFunnelHealth(context.Background(), common.GetTimestamp(), model.FunnelEnvironmentProduction)
	require.ErrorIs(t, err, ErrGeiliFunnelHealthUnavailable)
}

func finishHealthMaintenanceTask(t *testing.T, status model.SystemTaskStatus, result any) *model.SystemTask {
	t.Helper()
	task, err := model.CreateSystemTask(model.SystemTaskTypeFunnelMaintenance, nil, nil)
	require.NoError(t, err)
	runnerID := "health-runner"
	_, claimed, err := model.ClaimSystemTask(task.ID, task.Type, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	errorMessage := ""
	if status == model.SystemTaskStatusFailed {
		errorMessage = "dynamic private database error"
	}
	require.NoError(t, model.FinishSystemTask(task.TaskID, runnerID, status, result, errorMessage))
	return task
}
